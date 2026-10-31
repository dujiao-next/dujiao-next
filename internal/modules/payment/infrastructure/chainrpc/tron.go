package chainrpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strings"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

type tronTransaction struct {
	ID         string `json:"txID"`
	RawDataHex string `json:"raw_data_hex"`
	Ret        []struct {
		ContractRet string `json:"contractRet"`
	} `json:"ret"`
}

type tronBlock struct {
	ID     string `json:"blockID"`
	Header struct {
		Raw struct {
			Number    *int64 `json:"number"`
			Timestamp *int64 `json:"timestamp"`
		} `json:"raw_data"`
	} `json:"block_header"`
	Transactions []tronTransaction `json:"transactions"`
}

func (v *Verifier) solidBlock(ctx context.Context, s Source, path string, input interface{}) (tronBlock, uint64, int64, error) {
	var block tronBlock
	if err := v.post(ctx, s, path, input, &block); err != nil {
		return block, 0, 0, err
	}
	raw := block.Header.Raw
	if raw.Number == nil || *raw.Number < 0 || raw.Timestamp == nil || *raw.Timestamp <= 0 || canonicalHash(block.ID, false) == "" {
		return block, 0, 0, paymentcontract.ErrChainUnavailable
	}
	hash, _ := hex.DecodeString(block.ID)
	if binary.BigEndian.Uint64(hash[:8]) != uint64(*raw.Number) {
		return block, 0, 0, paymentcontract.ErrChainMismatch
	}
	return block, uint64(*raw.Number), *raw.Timestamp, nil
}

func (v *Verifier) verifyTRON(ctx context.Context, s Source, p NetworkPolicy, e paymentdomain.ChainExpectation, txID string) (*paymentdomain.ChainProof, error) {
	anchor, an, _, err := v.solidBlock(ctx, s, "/walletsolidity/getblockbynum", map[string]interface{}{"num": p.AnchorNumber})
	if err != nil {
		return nil, err
	}
	if an != p.AnchorNumber || canonicalHash(anchor.ID, false) != canonicalHash(p.AnchorHash, false) {
		return nil, paymentcontract.ErrChainMismatch
	}
	var tx tronTransaction
	if err := v.post(ctx, s, "/walletsolidity/gettransactionbyid", map[string]string{"value": txID}, &tx); err != nil {
		return nil, err
	}
	if tx.ID == "" {
		return nil, paymentcontract.ErrChainPending
	}
	if canonicalHash(tx.ID, false) != txID || len(tx.Ret) != 1 || tx.Ret[0].ContractRet != "SUCCESS" || len(tx.RawDataHex) > 1<<20 {
		return nil, paymentcontract.ErrChainMismatch
	}
	raw, err := hex.DecodeString(tx.RawDataHex)
	if err != nil || len(raw) == 0 {
		return nil, paymentcontract.ErrChainMismatch
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != txID {
		return nil, paymentcontract.ErrChainMismatch
	}
	var info struct {
		ID          string `json:"id"`
		BlockNumber *int64 `json:"blockNumber"`
		BlockTime   *int64 `json:"blockTimeStamp"`
		Result      string `json:"result"`
		Receipt     *struct {
			Result string `json:"result"`
		} `json:"receipt"`
		Logs []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"log"`
	}
	if err := v.post(ctx, s, "/walletsolidity/gettransactioninfobyid", map[string]string{"value": txID}, &info); err != nil {
		return nil, err
	}
	if info.ID == "" {
		return nil, paymentcontract.ErrChainPending
	}
	if canonicalHash(info.ID, false) != txID || info.BlockNumber == nil || *info.BlockNumber <= 0 || info.BlockTime == nil || info.Receipt == nil || info.Receipt.Result != "SUCCESS" || (info.Result != "" && info.Result != "SUCCESS") {
		return nil, paymentcontract.ErrChainMismatch
	}
	height := uint64(*info.BlockNumber)
	block, bn, timestamp, err := v.solidBlock(ctx, s, "/walletsolidity/getblockbynum", map[string]interface{}{"num": height})
	if err != nil {
		return nil, err
	}
	if bn != height || timestamp != *info.BlockTime || !tronBlockContains(block, txID) {
		return nil, paymentcontract.ErrChainMismatch
	}
	solidBlock, solid, solidTime, err := v.solidBlock(ctx, s, "/walletsolidity/getnowblock", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if !v.freshHead(solidTime) {
		return nil, paymentcontract.ErrChainUnavailable
	}
	if solid < height || solidTime < timestamp {
		return nil, paymentcontract.ErrChainPending
	}
	if solid < p.AnchorNumber || (solid == height && (canonicalHash(solidBlock.ID, false) != canonicalHash(block.ID, false) || solidTime != timestamp)) {
		return nil, paymentcontract.ErrChainMismatch
	}

	var matches []uint64
	for index, log := range info.Logs {
		if strings.ToLower(log.Address) != tronAddress(e.Contract) || len(log.Topics) == 0 || strings.ToLower(log.Topics[0]) != transferTopic {
			continue
		}
		if len(log.Topics) != 3 {
			return nil, paymentcontract.ErrChainMismatch
		}
		from, to, amount, ok := decodeTransfer(log.Topics[1], log.Topics[2], log.Data, false)
		if !ok || from == strings.Repeat("0", 40) {
			return nil, paymentcontract.ErrChainMismatch
		}
		if from == tronAddress(e.Recipient) {
			return nil, paymentcontract.ErrChainAmbiguous
		}
		if to == tronAddress(e.Recipient) && amount == e.RawAmount {
			matches = append(matches, uint64(index))
		}
	}
	if len(matches) == 0 {
		return nil, paymentcontract.ErrChainMismatch
	}
	if len(matches) != 1 {
		return nil, paymentcontract.ErrChainAmbiguous
	}
	if err := v.checkWindow(e, timestamp); err != nil {
		return nil, err
	}
	again, againNumber, againTime, err := v.solidBlock(ctx, s, "/walletsolidity/getblockbynum", map[string]interface{}{"num": height})
	if err != nil {
		return nil, err
	}
	if againNumber != height || againTime != timestamp || canonicalHash(again.ID, false) != canonicalHash(block.ID, false) || !tronBlockContains(again, txID) {
		return nil, paymentcontract.ErrChainDisagreement
	}
	return &paymentdomain.ChainProof{Network: e.Network, TransactionHash: txID, EventIndex: matches[0], BlockHash: canonicalHash(block.ID, false), BlockNumber: height, BlockTimeMs: timestamp, Contract: e.Contract, Recipient: e.Recipient, RawAmount: e.RawAmount, Finality: []paymentdomain.SourceFinality{{Name: s.Name, Provider: s.Provider, AnchorNumber: p.AnchorNumber, AnchorHash: canonicalHash(p.AnchorHash, false), Number: solid, Hash: canonicalHash(solidBlock.ID, false), TimeMs: solidTime}}}, nil
}

func tronBlockContains(b tronBlock, txID string) bool {
	matches := 0
	for _, tx := range b.Transactions {
		if canonicalHash(tx.ID, false) == txID {
			matches++
		}
	}
	return matches == 1
}

// Decode the merchant's checksummed Base58 address to a 20-byte event address.
func tronAddress(s string) string {
	if len(s) != 34 {
		return ""
	}
	n := new(big.Int)
	for _, c := range s {
		v := strings.IndexRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", c)
		if v < 0 {
			return ""
		}
		n.Mul(n, big.NewInt(58)).Add(n, big.NewInt(int64(v)))
	}
	b := n.Bytes()
	if len(b) != 25 || b[0] != 0x41 || bytes.Equal(b[1:21], make([]byte, 20)) {
		return ""
	}
	h := sha256.Sum256(b[:21])
	h = sha256.Sum256(h[:])
	if !bytes.Equal(b[21:], h[:4]) {
		return ""
	}
	return hex.EncodeToString(b[1:21])
}
