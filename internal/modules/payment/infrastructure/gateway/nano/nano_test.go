package nano

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Two public accounts with well-known keys: the burn account (all-zero key)
// and the genesis account.
const (
	burn       = "nano_1111111111111111111111111111111111111111111111111111hifc8npp"
	genesis    = "nano_3t6k35gi95xu6tergt6p69ck76ogmitsa8mnijtpxm9fkcm736xtoncuohr3"
	genesisKey = "E89208DD038FBB269987689621D52292AE9C35941A7484756ECCED92A65093BA"
	sendHash   = "991CF190094C00F0B68E2E5F75F6BEE95A2E0BD93CEAA4A6734DB9F19B728948"
)

func TestEncodeAddressKnownKeys(t *testing.T) {
	got, err := EncodeAddress(make([]byte, 32))
	if err != nil || got != burn {
		t.Fatalf("burn: %q %v", got, err)
	}
	key, _ := hex.DecodeString(genesisKey)
	got, err = EncodeAddress(key)
	if err != nil || got != genesis {
		t.Fatalf("genesis: %q %v", got, err)
	}
}

func TestParseAddress(t *testing.T) {
	for _, ok := range []string{burn, genesis, " " + genesis + "\n"} {
		if a, err := ParseAddress(ok); err != nil || a != strings.TrimSpace(ok) {
			t.Errorf("ParseAddress(%q) = %q, %v", ok, a, err)
		}
	}
	if a, err := ParseAddress("xrb_" + genesis[5:]); err != nil || a != genesis {
		t.Errorf("xrb_ form: %q %v", a, err)
	}
	flip := []byte(genesis)
	if flip[20] == '1' {
		flip[20] = '3'
	} else {
		flip[20] = '1'
	}
	bad := []string{
		"",
		genesis[5:],                            // no prefix
		"nano_" + genesis[6:],                  // short
		genesis[:len(genesis)-1] + "4",         // checksum
		string(flip),                           // key
		"nano_" + strings.Repeat("2", 60),      // '2' is not in the alphabet
		"nano_4" + genesis[6:],                 // padding bits set
		"NANO_" + strings.ToUpper(genesis[5:]), // case
	}
	for _, b := range bad {
		if _, err := ParseAddress(b); !errors.Is(err, ErrAddress) {
			t.Errorf("ParseAddress(%q) err = %v, want ErrAddress", b, err)
		}
	}
}

func TestExpectedRawIsDerivedAndBound(t *testing.T) {
	// 1.5 XNO priced at 6 decimals is 1_500_000 minor units.
	a, err := ExpectedRaw(1_500_000, 6, "order-a")
	if err != nil {
		t.Fatal(err)
	}
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)
	base := new(big.Int).Mul(big.NewInt(1_500_000), unit)
	tag := new(big.Int).Sub(a, base)
	if tag.Sign() <= 0 || tag.Cmp(unit) >= 0 {
		t.Fatalf("tag %s is not within one minor unit", tag)
	}
	if m, _ := MinorOf(a, 6); m != 1_500_000 {
		t.Fatalf("MinorOf = %d, the tag changed the price", m)
	}
	again, _ := ExpectedRaw(1_500_000, 6, "order-a")
	if again.Cmp(a) != 0 {
		t.Fatal("same order, different amount")
	}
	b, _ := ExpectedRaw(1_500_000, 6, "order-b")
	if b.Cmp(a) == 0 {
		t.Fatal("two orders share one amount")
	}
	// 1 XNO is 10^30 raw, whatever scale it is priced at.
	one, _ := ExpectedRaw(100, 2, "x")
	if m, _ := MinorOf(one, 2); m != 100 || len(one.String()) != 31 {
		t.Fatalf("1 XNO at 2 decimals = %s", one)
	}
}

func TestExpectedRawRefuses(t *testing.T) {
	if _, err := ExpectedRaw(0, 6, "o"); !errors.Is(err, ErrAmount) {
		t.Error("zero amount accepted")
	}
	if _, err := ExpectedRaw(-5, 6, "o"); !errors.Is(err, ErrAmount) {
		t.Error("negative amount accepted")
	}
	if _, err := ExpectedRaw(5, 6, ""); err == nil {
		t.Error("empty reference accepted")
	}
	if _, err := ExpectedRaw(5, 19, "o"); !errors.Is(err, ErrScale) {
		t.Error("19 decimals leaves 11 tag digits; want ErrScale")
	}
}

func sendInfo(to, amount string) BlockInfo {
	var b BlockInfo
	b.BlockAccount = burn
	b.Amount = amount
	b.Confirmed = "true"
	b.Subtype = "send"
	b.Contents.Type = "state"
	b.Contents.LinkAsAccount = to
	return b
}

func TestCheckSend(t *testing.T) {
	want, _ := ExpectedRaw(2_000_000, 6, "order-1")
	ok := sendInfo(genesis, want.String())
	p, err := CheckSend(strings.ToLower(sendHash), ok, genesis, want)
	if err != nil || p.Hash != sendHash || p.Payer != burn || p.Amount.Cmp(want) != 0 {
		t.Fatalf("good send refused: %+v %v", p, err)
	}

	more := new(big.Int).Add(want, big.NewInt(1))
	less := new(big.Int).Sub(want, big.NewInt(1))
	other, _ := ExpectedRaw(2_000_000, 6, "order-2")
	cases := []struct {
		name string
		info BlockInfo
		want error
	}{
		{"one raw more", sendInfo(genesis, more.String()), ErrWrongValue},
		{"one raw less", sendInfo(genesis, less.String()), ErrWrongValue},
		{"another order's amount", sendInfo(genesis, other.String()), ErrWrongValue},
		{"other recipient", sendInfo(burn, want.String()), ErrRecipient},
		{"unconfirmed", func() BlockInfo { b := ok; b.Confirmed = "false"; return b }(), ErrNotSend},
		{"receive", func() BlockInfo { b := ok; b.Subtype = "receive"; return b }(), ErrNotSend},
		{"legacy block", func() BlockInfo { b := ok; b.Contents.Type = "send"; return b }(), ErrNotSend},
	}
	for _, c := range cases {
		if _, err := CheckSend(sendHash, c.info, genesis, want); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := CheckSend("zz", ok, genesis, want); !errors.Is(err, ErrBlockHash) {
		t.Errorf("bad hash: %v", err)
	}
}

func TestClaimsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	if dup, err := (FileClaims{Dir: dir}).Claim(sendHash, "order-1"); err != nil || dup {
		t.Fatalf("first claim: %v %v", dup, err)
	}
	// A new value, as after a process restart: only the directory is shared.
	again := FileClaims{Dir: dir}
	if dup, err := again.Claim(strings.ToLower(sendHash), "order-1"); err != nil || !dup {
		t.Fatalf("same order again: dup=%v err=%v", dup, err)
	}
	if _, err := again.Claim(sendHash, "order-2"); !errors.Is(err, ErrReplayed) {
		t.Fatalf("replay for another order: %v", err)
	}
	if _, err := (FileClaims{}).Claim(sendHash, "o"); err == nil {
		t.Fatal("claim without a directory accepted")
	}
}

func TestClaimsRace(t *testing.T) {
	dir := t.TempDir()
	var won int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			order := "order-" + string(rune('a'+i))
			if dup, err := (FileClaims{Dir: dir}).Claim(sendHash, order); err == nil && !dup {
				atomic.AddInt32(&won, 1)
			}
		}(i)
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d orders settled by one block, want 1", won)
	}
}

func fakeNode(t *testing.T, answers map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		for k := range req {
			if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "seed") || k == "wallet" {
				t.Errorf("node request carries %q", k)
			}
		}
		a, ok := answers[req["action"]]
		if !ok {
			a = map[string]string{"error": "Block not found"}
		}
		json.NewEncoder(w).Encode(a)
	}))
}

func TestNodeBlockInfoAndFindSend(t *testing.T) {
	want, _ := ExpectedRaw(3, 6, "order-n")
	srv := fakeNode(t, map[string]interface{}{
		"block_info": sendInfo(genesis, want.String()),
		"receivable": map[string]interface{}{"blocks": map[string]interface{}{
			strings.Repeat("A", 64): map[string]string{"amount": "1", "source": burn},
		}},
		"account_history": map[string]interface{}{"history": []map[string]string{
			{"type": "state", "subtype": "receive", "amount": want.String(), "link": sendHash},
		}},
	})
	defer srv.Close()
	n := Node{URL: srv.URL}
	ctx := context.Background()
	info, err := n.BlockInfo(ctx, sendHash)
	if err != nil || info.Amount != want.String() {
		t.Fatalf("BlockInfo: %+v %v", info, err)
	}
	h, err := n.FindSend(ctx, genesis, want, 0)
	if err != nil || h != sendHash {
		t.Fatalf("FindSend = %q %v", h, err)
	}
	h, err = n.FindSend(ctx, genesis, big.NewInt(42), 0)
	if err != nil || h != "" {
		t.Fatalf("FindSend absent = %q %v", h, err)
	}
	if _, err := (Node{URL: srv.URL}).BlockInfo(ctx, "nothex"); !errors.Is(err, ErrBlockHash) {
		t.Fatalf("bad hash sent to node: %v", err)
	}
	empty := fakeNode(t, map[string]interface{}{})
	defer empty.Close()
	if _, err := (Node{URL: empty.URL}).BlockInfo(ctx, sendHash); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("missing block: %v", err)
	}
	if _, err := (Node{}).BlockInfo(ctx, sendHash); err == nil {
		t.Fatal("no node url accepted")
	}
}
