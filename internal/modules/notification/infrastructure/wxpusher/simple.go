package wxpusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/modules/support/contract"
)

func (s *Sender) SendSimple(ctx context.Context, spt, summary, content, link string) (contract.PushResult, error) {
	if !strings.HasPrefix(spt, "SPT_") || len(spt) > 512 {
		return contract.PushResult{}, fmt.Errorf("wxpusher SPT invalid")
	}
	body, _ := json.Marshal(map[string]interface{}{"spt": spt, "summary": summary, "content": content, "contentType": 1, "url": link})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.providerURL("/api/send/message/simple-push"), bytes.NewReader(body))
	if err != nil {
		return contract.PushResult{}, fmt.Errorf("wxpusher request invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return contract.PushResult{}, fmt.Errorf("wxpusher network failure")
	}
	defer resp.Body.Close()
	var v struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&v) != nil || resp.StatusCode != 200 || v.Code != 1000 {
		return contract.PushResult{}, fmt.Errorf("wxpusher simple push rejected")
	}
	// Some versions return per-recipient results; reject any reported failure.
	var results []struct {
		Code         int   `json:"code"`
		SendRecordID int64 `json:"sendRecordId"`
	}
	if len(v.Data) > 0 && v.Data[0] == '[' {
		if json.Unmarshal(v.Data, &results) != nil || len(results) == 0 {
			return contract.PushResult{}, fmt.Errorf("wxpusher simple response invalid")
		}
		for _, r := range results {
			if r.Code != 1000 {
				return contract.PushResult{}, fmt.Errorf("wxpusher simple recipient rejected")
			}
		}
		return contract.PushResult{SendRecordID: results[0].SendRecordID}, nil
	}
	return contract.PushResult{}, nil
}
