package epusdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

func TestCandidateProtocolLimits(t *testing.T) {
	row := paymentdomain.TransferCandidate{ID: "1", ObservationKey: strings.Repeat("a", 64), OrderID: strings.Repeat("o", 32), TradeID: strings.Repeat("t", 32),
		Network: "binance", Kind: "evm_log", TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64),
		Contract: "0x" + strings.Repeat("3", 40), Recipient: "0x" + strings.Repeat("4", 40), RawAmount: "14000100000000000000", ReviewReason: "independent_check_required"}
	for _, scenario := range []string{"full batch", "too many", "duplicate", "noncanonical id", "numeric id", "null data", "oversize", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "redirect" {
					http.Redirect(w, r, "/must-not-follow", 302)
					return
				}
				if scenario == "oversize" {
					_, _ = w.Write([]byte(strings.Repeat(" ", 512<<10) + "{}"))
					return
				}
				if scenario == "numeric id" {
					_, _ = w.Write([]byte(`{"status_code":200,"data":[{"id":1}]}`))
					return
				}
				var rows []paymentdomain.TransferCandidate
				if scenario != "null data" {
					for i := 1; i <= 100; i++ {
						c := row
						c.ID = strconv.Itoa(i)
						rows = append(rows, c)
					}
					if scenario == "too many" {
						c := row
						c.ID = "101"
						rows = append(rows, c)
					}
					if scenario == "duplicate" {
						rows[1].ID = rows[0].ID
					}
					if scenario == "noncanonical id" {
						rows[0].ID = "01"
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status_code": 200, "data": rows})
			}))
			defer srv.Close()
			cfg := quoteConfig("binance")
			cfg.GatewayURL = srv.URL
			rows, err := ListCandidates(context.Background(), cfg)
			if scenario == "full batch" {
				if err != nil || len(rows) != 100 {
					t.Fatalf("valid 100-row batch rejected: %v", err)
				}
			} else if err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}
