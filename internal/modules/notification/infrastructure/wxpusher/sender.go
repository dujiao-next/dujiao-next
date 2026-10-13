// Package wxpusher implements standard UID-targeted push, not chat transport.
package wxpusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dujiao-next/internal/modules/support/contract"
)

type Sender struct {
	Client   *http.Client
	Endpoint string
}

func New() *Sender {
	return &Sender{Client: &http.Client{}, Endpoint: "https://wxpusher.zjiecode.com/api/send/message"}
}
func (s *Sender) Send(ctx context.Context, token, uid, summary, content, url string) (contract.PushResult, error) {
	if token == "" || uid == "" {
		return contract.PushResult{}, fmt.Errorf("wxpusher configuration missing")
	}
	body, _ := json.Marshal(map[string]interface{}{"appToken": token, "uids": []string{uid}, "summary": summary, "content": content, "contentType": 1, "url": url})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(body))
	if err != nil {
		return contract.PushResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return contract.PushResult{}, fmt.Errorf("wxpusher network failure")
	}
	defer resp.Body.Close()
	var v struct {
		Code int `json:"code"`
		Data []struct {
			Code         int   `json:"code"`
			SendRecordID int64 `json:"sendRecordId"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&v); err != nil {
		return contract.PushResult{}, fmt.Errorf("wxpusher invalid response")
	}
	if resp.StatusCode != 200 || v.Code != 1000 || len(v.Data) != 1 || v.Data[0].Code != 1000 {
		return contract.PushResult{}, fmt.Errorf("wxpusher rejected (http=%d code=%d)", resp.StatusCode, v.Code)
	}
	return contract.PushResult{SendRecordID: v.Data[0].SendRecordID}, nil
}
