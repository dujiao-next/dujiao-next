package chainrpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

func testHash(c string) string { return "0x" + strings.Repeat(c, 64) }
func tronTestBlockID(number uint64, c string) string {
	return fmt.Sprintf("%016x", number) + strings.Repeat(c, 48)
}

func bscFixture(t *testing.T, scenario string, now time.Time) *httptest.Server {
	t.Helper()
	var blockReads atomic.Int32
	txID, blockHash := testHash("1"), testHash("b")
	if scenario == "different canonical block" {
		blockHash = testHash("c")
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			w.WriteHeader(400)
			return
		}
		if scenario == "rpc unavailable" {
			w.WriteHeader(429)
			return
		}
		var result interface{}
		switch req.Method {
		case "eth_chainId":
			result = "0x38"
			if scenario == "wrong chain" {
				result = "0x61"
			}
		case "eth_getTransactionByHash":
			result = map[string]interface{}{"hash": txID, "blockHash": blockHash, "blockNumber": "0x64", "transactionIndex": "0x0"}
			if scenario == "wrong transaction" {
				result.(map[string]interface{})["hash"] = testHash("2")
			}
		case "eth_getBlockByNumber":
			var tag string
			_ = json.Unmarshal(req.Params[0], &tag)
			hash, number, timestamp := blockHash, "0x64", now.Add(-5*time.Second).Unix()
			transactions := []string{txID}
			switch tag {
			case "0x1":
				hash, number, timestamp = testHash("a"), "0x1", now.Add(-time.Hour).Unix()
				if scenario == "wrong anchor" {
					hash = testHash("d")
				}
			case "finalized":
				hash, number, timestamp = testHash("f"), "0x65", now.Unix()
				if scenario == "conflicting final head" {
					number = "0x64"
					timestamp = now.Add(-5 * time.Second).Unix()
				}
				if scenario == "not final" {
					number = "0x63"
				}
				if scenario == "stale head" {
					timestamp = now.Add(-time.Hour).Unix()
				}
				if scenario == "future head" {
					timestamp = now.Add(time.Hour).Unix()
				}
				if scenario == "unsupported finality" {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "error": map[string]interface{}{"code": -32602}})
					return
				}
			case "0x64":
				if scenario == "missing inclusion" {
					transactions = []string{testHash("2")}
				}
				if blockReads.Add(1) > 1 && scenario == "reorg" {
					hash = testHash("d")
				}
			default:
				t.Errorf("unexpected block tag %s", tag)
			}
			result = map[string]interface{}{"hash": hash, "number": number, "timestamp": fmt.Sprintf("0x%x", timestamp), "transactions": transactions}
		case "eth_getTransactionReceipt":
			removed := false
			log := map[string]interface{}{"address": bscUSDT, "topics": []string{"0x" + transferTopic, "0x" + strings.Repeat("0", 24) + strings.Repeat("2", 40), "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40)},
				"data": fmt.Sprintf("0x%064x", uint64(1000000000000000000)), "transactionHash": txID, "transactionIndex": "0x0", "blockHash": blockHash, "blockNumber": "0x64", "logIndex": "0x2", "removed": removed}
			status := "0x1"
			if scenario == "failed transaction" {
				status = "0x0"
			}
			switch scenario {
			case "wrong token":
				log["address"] = "0x" + strings.Repeat("3", 40)
			case "wrong recipient":
				log["topics"].([]string)[2] = "0x" + strings.Repeat("0", 24) + strings.Repeat("3", 40)
			case "wrong amount":
				log["data"] = fmt.Sprintf("0x%064x", uint64(1))
			case "malformed amount":
				log["data"] = "0x01"
			case "self transfer":
				log["topics"].([]string)[1] = log["topics"].([]string)[2]
			case "bad address padding":
				log["topics"].([]string)[2] = "0x" + strings.Repeat("1", 64)
			case "removed":
				log["removed"] = true
			case "missing removed":
				delete(log, "removed")
			case "wrong log transaction":
				log["transactionHash"] = testHash("3")
			case "wrong log block":
				log["blockHash"] = testHash("3")
			case "missing index":
				delete(log, "logIndex")
			}
			logs := []interface{}{log}
			if scenario == "no transfer" {
				logs = nil
			}
			if scenario == "multiple transfers" || scenario == "duplicate index" {
				second := map[string]interface{}{}
				for k, v := range log {
					second[k] = v
				}
				if scenario == "multiple transfers" {
					second["logIndex"] = "0x3"
				}
				logs = append(logs, second)
			}
			result = map[string]interface{}{"transactionHash": txID, "transactionIndex": "0x0", "status": status, "blockHash": blockHash, "blockNumber": "0x64", "logs": logs}
			if scenario == "missing receipt" {
				result = nil
			}
		default:
			t.Errorf("unexpected RPC %s", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
}

func TestBSCIndependentVerification(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	e := paymentdomain.ChainExpectation{Network: "binance", Contract: bscUSDT, Recipient: "0x" + strings.Repeat("1", 40), RawAmount: "1000000000000000000", CreatedAtMs: now.Add(-time.Minute).UnixMilli(), ExpiresAtMs: now.Add(-time.Second).UnixMilli()}
	for _, scenario := range []string{"valid after expiry", "wrong chain", "wrong anchor", "failed transaction", "wrong transaction", "missing inclusion", "wrong token", "wrong recipient", "wrong amount", "malformed amount", "self transfer", "bad address padding", "removed", "missing removed", "wrong log transaction", "wrong log block", "missing index", "duplicate index", "no transfer", "multiple transfers", "conflicting final head", "not final", "stale head", "future head", "unsupported finality", "rpc unavailable", "missing receipt", "reorg", "different canonical block", "late"} {
		t.Run(scenario, func(t *testing.T) {
			first := bscFixture(t, "", now)
			defer first.Close()
			second := bscFixture(t, scenario, now)
			defer second.Close()
			v := &Verifier{policy: Policy{Version: "test-v1", MaxHeadAge: time.Minute, BSC: NetworkPolicy{AnchorNumber: 1, AnchorHash: testHash("a"), Sources: []Source{{Name: "first", Provider: "one", URL: first.URL}, {Name: "second", Provider: "two", URL: second.URL}}}}, client: first.Client(), now: func() time.Time { return now }}
			expected := e
			if scenario == "late" {
				expected.ExpiresAtMs = now.Add(-5 * time.Second).UnixMilli()
			}
			proof, err := v.Verify(context.Background(), expected, testHash("1"))
			if scenario == "valid after expiry" {
				if err != nil || proof == nil || proof.EventIndex != 2 || len(proof.Finality) != 2 || proof.BlockTimeMs != now.Add(-5*time.Second).UnixMilli() {
					t.Fatalf("valid payment rejected: %v %#v", err, proof)
				}
				return
			}
			if err == nil || proof != nil {
				t.Fatal("unsafe proof accepted")
			}
			if scenario == "different canonical block" && !errors.Is(err, paymentcontract.ErrChainDisagreement) {
				t.Fatalf("source disagreement not classified: %v", err)
			}
			if scenario == "late" && !errors.Is(err, paymentcontract.ErrChainOutsideWindow) {
				t.Fatalf("late not classified: %v", err)
			}
		})
	}
}

func tronFixture(t *testing.T, scenario string, now time.Time) *httptest.Server {
	t.Helper()
	var blockReads atomic.Int32
	raw := []byte{10, 0}
	h := sha256.Sum256(raw)
	txID := hex.EncodeToString(h[:])
	blockHash := tronTestBlockID(100, "b")
	if scenario == "different canonical block" {
		blockHash = tronTestBlockID(100, "c")
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scenario == "rpc unavailable" {
			w.WriteHeader(429)
			return
		}
		var input map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&input)
		var result interface{}
		switch r.URL.Path {
		case "/walletsolidity/gettransactionbyid":
			ret := "SUCCESS"
			if scenario == "failed transaction" {
				ret = "REVERT"
			}
			result = map[string]interface{}{"txID": txID, "raw_data_hex": hex.EncodeToString(raw), "ret": []interface{}{map[string]string{"contractRet": ret}}}
			if scenario == "wrong raw hash" {
				result.(map[string]interface{})["raw_data_hex"] = "010203"
			}
			if scenario == "missing transaction success" {
				delete(result.(map[string]interface{}), "ret")
			}
		case "/walletsolidity/gettransactioninfobyid":
			log := map[string]interface{}{"address": tronAddress(tronUSDT), "topics": []string{transferTopic, strings.Repeat("0", 24) + strings.Repeat("2", 40), strings.Repeat("0", 24) + tronAddress("TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV")}, "data": fmt.Sprintf("%064x", 1000000)}
			switch scenario {
			case "wrong token":
				log["address"] = strings.Repeat("3", 40)
			case "wrong recipient":
				log["topics"].([]string)[2] = strings.Repeat("0", 24) + strings.Repeat("3", 40)
			case "wrong amount":
				log["data"] = fmt.Sprintf("%064x", 1)
			case "self transfer":
				log["topics"].([]string)[1] = log["topics"].([]string)[2]
			case "bad address padding":
				log["topics"].([]string)[2] = strings.Repeat("1", 64)
			}
			logs := []interface{}{log}
			if scenario == "no transfer" {
				logs = nil
			}
			if scenario == "multiple transfers" {
				logs = append(logs, log)
			}
			result = map[string]interface{}{"id": txID, "blockNumber": 100, "blockTimeStamp": now.Add(-5 * time.Second).UnixMilli(), "receipt": map[string]string{"result": "SUCCESS"}, "log": logs}
			switch scenario {
			case "VM failure":
				result.(map[string]interface{})["receipt"] = map[string]string{"result": "REVERT"}
			case "missing VM success":
				result.(map[string]interface{})["receipt"] = map[string]string{}
			case "failed result":
				result.(map[string]interface{})["result"] = "FAILED"
			case "wrong receipt id":
				result.(map[string]interface{})["id"] = strings.Repeat("2", 64)
			case "wrong receipt time":
				result.(map[string]interface{})["blockTimeStamp"] = now.UnixMilli()
			case "missing receipt":
				result = map[string]interface{}{}
			}
		case "/walletsolidity/getblockbynum", "/walletsolidity/getnowblock":
			height, hash, timestamp := uint64(100), blockHash, now.Add(-5*time.Second).UnixMilli()
			transactions := []interface{}{map[string]string{"txID": txID}}
			if r.URL.Path == "/walletsolidity/getnowblock" {
				height, hash, timestamp = 101, tronTestBlockID(101, "f"), now.UnixMilli()
				if scenario == "conflicting final head" {
					height, hash, timestamp = 100, tronTestBlockID(100, "f"), now.Add(-5*time.Second).UnixMilli()
				}
				if scenario == "not solid" {
					height, hash = 99, tronTestBlockID(99, "f")
				}
				if scenario == "stale head" {
					timestamp = now.Add(-time.Hour).UnixMilli()
				}
			} else if input["num"] == float64(1) {
				height, hash, timestamp = 1, tronTestBlockID(1, "a"), now.Add(-time.Hour).UnixMilli()
				if scenario == "wrong anchor" {
					hash = tronTestBlockID(1, "d")
				}
			} else {
				if scenario == "missing inclusion" {
					transactions = nil
				}
				if scenario == "wrong block height" {
					hash = tronTestBlockID(98, "b")
				}
				if blockReads.Add(1) > 1 && scenario == "reorg" {
					hash = tronTestBlockID(100, "d")
				}
			}
			result = map[string]interface{}{"blockID": hash, "block_header": map[string]interface{}{"raw_data": map[string]interface{}{"number": height, "timestamp": timestamp}}, "transactions": transactions}
		default:
			t.Errorf("non-solid or unexpected endpoint %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
}

func TestTRONIndependentVerification(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	hash := sha256.Sum256([]byte{10, 0})
	txID := hex.EncodeToString(hash[:])
	e := paymentdomain.ChainExpectation{Network: "tron", Contract: tronUSDT, Recipient: "TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV", RawAmount: "1000000", CreatedAtMs: now.Add(-time.Minute).UnixMilli(), ExpiresAtMs: now.Add(-time.Second).UnixMilli()}
	for _, scenario := range []string{"valid after expiry", "wrong anchor", "failed transaction", "missing transaction success", "wrong raw hash", "VM failure", "missing VM success", "failed result", "wrong receipt id", "wrong receipt time", "missing receipt", "wrong token", "wrong recipient", "wrong amount", "self transfer", "bad address padding", "no transfer", "multiple transfers", "missing inclusion", "wrong block height", "conflicting final head", "not solid", "stale head", "rpc unavailable", "reorg", "different canonical block", "late"} {
		t.Run(scenario, func(t *testing.T) {
			first := tronFixture(t, "", now)
			defer first.Close()
			second := tronFixture(t, scenario, now)
			defer second.Close()
			v := &Verifier{policy: Policy{Version: "test-v1", MaxHeadAge: time.Minute, TRON: NetworkPolicy{AnchorNumber: 1, AnchorHash: tronTestBlockID(1, "a"), Sources: []Source{{Name: "first", Provider: "one", URL: first.URL}, {Name: "second", Provider: "two", URL: second.URL}}}}, client: first.Client(), now: func() time.Time { return now }}
			expected := e
			if scenario == "late" {
				expected.ExpiresAtMs = now.Add(-5 * time.Second).UnixMilli()
			}
			proof, err := v.Verify(context.Background(), expected, txID)
			if scenario == "valid after expiry" {
				if err != nil || proof == nil || proof.EventIndex != 0 || len(proof.Finality) != 2 {
					t.Fatalf("valid transfer rejected: %v %#v", err, proof)
				}
				return
			}
			if err == nil || proof != nil {
				t.Fatal("unsafe proof accepted")
			}
			if scenario == "different canonical block" && !errors.Is(err, paymentcontract.ErrChainDisagreement) {
				t.Fatalf("disagreement not classified: %v", err)
			}
		})
	}
}

func TestRPCPolicyAndPublicAddressRestrictions(t *testing.T) {
	base := Policy{Version: "test", MaxHeadAge: time.Minute, BSC: NetworkPolicy{AnchorNumber: 1, AnchorHash: testHash("a"), Sources: []Source{{Name: "a", Provider: "one", URL: "https://rpc.one.example/key"}, {Name: "b", Provider: "two", URL: "https://rpc.two.example"}}}, TRON: NetworkPolicy{AnchorNumber: 1, AnchorHash: tronTestBlockID(1, "a"), Sources: []Source{{Name: "a", Provider: "one", URL: "https://tron.one.example"}, {Name: "b", Provider: "two", URL: "https://tron.two.example"}}}}
	if _, err := New(base); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"same provider", "same host", "one source", "http", "credentials", "query", "missing anchor"} {
		p := base
		p.BSC.Sources = append([]Source(nil), base.BSC.Sources...)
		switch scenario {
		case "same provider":
			p.BSC.Sources[1].Provider = " ONE "
		case "same host":
			p.BSC.Sources[1].URL = "https://rpc.one.example/other"
		case "one source":
			p.BSC.Sources = p.BSC.Sources[:1]
		case "http":
			p.BSC.Sources[0].URL = "http://rpc.one.example"
		case "credentials":
			p.BSC.Sources[0].URL = "https://user:password@rpc.one.example"
		case "query":
			p.BSC.Sources[0].URL = "https://rpc.one.example?key=secret"
		case "missing anchor":
			p.BSC.AnchorHash = ""
		}
		if _, err := New(p); err == nil {
			t.Errorf("unsafe policy accepted: %s", scenario)
		}
	}
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1"} {
		if publicRPCAddress(netip.MustParseAddr(s)) {
			t.Errorf("nonpublic RPC allowed %s", s)
		}
	}
	if !publicRPCAddress(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public address rejected")
	}
	if tronAddress(tronUSDT) != "a614f803b6fd780986a42c78ec9c7f77e6ded13c" {
		t.Fatal("pinned TRON USDT contract decoding changed")
	}
}

func TestRPCTransportBoundsAndRedaction(t *testing.T) {
	for _, scenario := range []string{"redirect", "oversize", "wrong response id", "RPC error"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch scenario {
				case "redirect":
					http.Redirect(w, r, "/followed", 302)
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat(" ", 4<<20) + "{}"))
				case "wrong response id":
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":"0x38"}`))
				case "RPC error":
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"message":"credential=secret-test-key"}}`))
				}
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = rpcHTTPClient().CheckRedirect
			v := &Verifier{client: client}
			var out string
			err := v.evmCall(context.Background(), Source{URL: server.URL + "/secret-test-key"}, "eth_chainId", []interface{}{}, &out)
			if err == nil || strings.Contains(err.Error(), "secret-test-key") || calls.Load() != 1 {
				t.Fatalf("unsafe transport response: %v calls=%d", err, calls.Load())
			}
		})
	}
}
