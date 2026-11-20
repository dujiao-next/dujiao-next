// Package nano 是 Nano (XNO) 收款的纯逻辑部分：地址校验、订单的精确 raw 金额、
// 以及对节点返回的一个 send 区块的校验。只依赖标准库和 x/crypto/blake2b，
// 可以脱离仓库其余依赖单独测试。
//
// 这里不持有任何私钥或种子。网关只从运营者配置的节点读取区块；付款由买家钱包完成。
package nano

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// RawDecimals is the scale of Nano's smallest unit: 1 XNO is 10^30 raw.
const RawDecimals = 30

// minTagDigits is how many raw digits below the priced minor unit must be free
// for the per-order tag. 10^12 distinct tags keeps two open orders of the same
// price from sharing an amount.
const minTagDigits = 12

const alphabet = "13456789abcdefghijkmnopqrstuwxyz"

var (
	ErrAddress    = errors.New("nano: address is not a valid nano_ account")
	ErrScale      = errors.New("nano: price scale leaves no room for the order tag")
	ErrAmount     = errors.New("nano: amount must be positive")
	ErrBlockHash  = errors.New("nano: block hash must be 64 hex characters")
	ErrNotSend    = errors.New("nano: block is not a confirmed send")
	ErrRecipient  = errors.New("nano: block pays another account")
	ErrWrongValue = errors.New("nano: block amount is not the order's exact amount")
)

// ParseAddress checks an account's checksum and returns it in nano_ form. The
// legacy xrb_ prefix is accepted and normalised.
func ParseAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	var body string
	switch {
	case strings.HasPrefix(s, "nano_"):
		body = s[len("nano_"):]
	case strings.HasPrefix(s, "xrb_"):
		body = s[len("xrb_"):]
	default:
		return "", ErrAddress
	}
	if len(body) != 60 || (body[0] != '1' && body[0] != '3') {
		return "", ErrAddress
	}
	bits := new(big.Int)
	for i := 0; i < 60; i++ {
		v := strings.IndexByte(alphabet, body[i])
		if v < 0 {
			return "", ErrAddress
		}
		bits.Lsh(bits, 5)
		bits.Or(bits, big.NewInt(int64(v)))
	}
	// 300 bits: 4 zero bits, 256 bits of public key, 40 bits of checksum.
	raw := bits.FillBytes(make([]byte, 38)) // 304 bits; the top 8 must be zero
	if raw[0] != 0 {
		return "", ErrAddress
	}
	b := new(big.Int).SetBytes(raw)
	check := new(big.Int).And(b, new(big.Int).SetUint64(1<<40-1))
	pub := new(big.Int).Rsh(b, 40).FillBytes(make([]byte, 32))
	want := checksum(pub)
	got := check.FillBytes(make([]byte, 5))
	for i := range want {
		if want[i] != got[i] {
			return "", ErrAddress
		}
	}
	return "nano_" + body, nil
}

// EncodeAddress renders a 32-byte public key as a nano_ account.
func EncodeAddress(pub []byte) (string, error) {
	if len(pub) != 32 {
		return "", ErrAddress
	}
	b := new(big.Int).SetBytes(pub)
	b.Lsh(b, 40)
	b.Or(b, new(big.Int).SetBytes(checksum(pub)))
	out := make([]byte, 60)
	mask := big.NewInt(31)
	for i := 59; i >= 0; i-- {
		out[i] = alphabet[new(big.Int).And(b, mask).Int64()]
		b.Rsh(b, 5)
	}
	return "nano_" + string(out), nil
}

// checksum is the 5-byte blake2b digest of the key, byte-reversed.
func checksum(pub []byte) []byte {
	h, _ := blake2b.New(5, nil)
	h.Write(pub)
	d := h.Sum(nil)
	for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
		d[i], d[j] = d[j], d[i]
	}
	return d
}

// tagSpace is 10^(30-decimals): the raw count of one priced minor unit.
func tagSpace(decimals int32) (*big.Int, error) {
	if decimals < 0 || RawDecimals-int(decimals) < minTagDigits {
		return nil, ErrScale
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(RawDecimals-int(decimals))), nil), nil
}

// ExpectedRaw is the exact amount, in raw, that settles one order: the price in
// minor units scaled to raw, plus a tag below one minor unit derived from the
// order's reference. The tag binds the payment to the order - a send made for
// one order cannot settle another - and leaves the displayed price unchanged.
func ExpectedRaw(minor int64, decimals int32, reference string) (*big.Int, error) {
	if minor <= 0 {
		return nil, ErrAmount
	}
	if reference == "" {
		return nil, errors.New("nano: order reference is empty")
	}
	unit, err := tagSpace(decimals)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte("dujiao-next-nano-payment:" + reference))
	tag := new(big.Int).SetBytes(sum[:])
	tag.Mod(tag, new(big.Int).Sub(unit, big.NewInt(1)))
	tag.Add(tag, big.NewInt(1)) // 1 .. unit-1, never zero
	raw := new(big.Int).Mul(big.NewInt(minor), unit)
	return raw.Add(raw, tag), nil
}

// MinorOf floors a raw amount to the priced minor unit.
func MinorOf(raw *big.Int, decimals int32) (int64, error) {
	unit, err := tagSpace(decimals)
	if err != nil {
		return 0, err
	}
	q := new(big.Int).Quo(raw, unit)
	if !q.IsInt64() {
		return 0, fmt.Errorf("nano: %s raw does not fit the minor unit", raw)
	}
	return q.Int64(), nil
}

// ParseRaw reads a non-negative decimal raw amount.
func ParseRaw(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return nil, fmt.Errorf("nano: %q is not a raw amount", s)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("nano: %q is not a raw amount", s)
	}
	return v, nil
}

// NormalizeHash checks a block hash and returns it upper-case.
func NormalizeHash(h string) (string, error) {
	h = strings.TrimSpace(h)
	if len(h) != 64 {
		return "", ErrBlockHash
	}
	if _, err := hex.DecodeString(h); err != nil {
		return "", ErrBlockHash
	}
	return strings.ToUpper(h), nil
}

// BlockInfo is the part of a node's block_info answer (json_block true) the
// check reads.
type BlockInfo struct {
	BlockAccount string `json:"block_account"`
	Amount       string `json:"amount"`
	Confirmed    string `json:"confirmed"`
	Subtype      string `json:"subtype"`
	Contents     struct {
		Type          string `json:"type"`
		LinkAsAccount string `json:"link_as_account"`
	} `json:"contents"`
}

// Paid is a send that settles an order.
type Paid struct {
	Hash   string
	Payer  string
	Amount *big.Int
}

// CheckSend accepts a block only when it is a confirmed state send to
// recipient of exactly want raw. More is refused as well as less: the amount
// is the order's binding, so a different amount is a different order.
func CheckSend(hash string, info BlockInfo, recipient string, want *big.Int) (*Paid, error) {
	h, err := NormalizeHash(hash)
	if err != nil {
		return nil, err
	}
	if info.Confirmed != "true" || info.Subtype != "send" || info.Contents.Type != "state" {
		return nil, ErrNotSend
	}
	to, err := ParseAddress(info.Contents.LinkAsAccount)
	if err != nil {
		return nil, ErrRecipient
	}
	rcpt, err := ParseAddress(recipient)
	if err != nil {
		return nil, err
	}
	if to != rcpt {
		return nil, ErrRecipient
	}
	got, err := ParseRaw(info.Amount)
	if err != nil {
		return nil, err
	}
	if want == nil || got.Cmp(want) != 0 {
		return nil, ErrWrongValue
	}
	payer, err := ParseAddress(info.BlockAccount)
	if err != nil {
		return nil, ErrNotSend
	}
	return &Paid{Hash: h, Payer: payer, Amount: got}, nil
}
