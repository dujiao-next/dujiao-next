package wxpusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/modules/support/contract"
)

func (s *Sender) providerURL(path string) string {
	base := strings.TrimSuffix(s.Endpoint, "/api/send/message")
	return base + path
}
func (s *Sender) CreateQRCode(ctx context.Context, token, extra string, validTime int) (contract.ParameterQRCode, error) {
	body, _ := json.Marshal(map[string]interface{}{"appToken": token, "extra": extra, "validTime": validTime})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.providerURL("/api/fun/create/qrcode"), bytes.NewReader(body))
	if err != nil {
		return contract.ParameterQRCode{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return contract.ParameterQRCode{}, fmt.Errorf("wxpusher QR request failed")
	}
	defer resp.Body.Close()
	var v struct {
		Code int `json:"code"`
		Data struct {
			Code string `json:"code"`
			URL  string `json:"url"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&v) != nil || resp.StatusCode != 200 || v.Code != 1000 || v.Data.Code == "" || len(v.Data.Code) > 512 {
		return contract.ParameterQRCode{}, fmt.Errorf("wxpusher QR response invalid")
	}
	u, err := url.Parse(v.Data.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "wxpusher.zjiecode.com" || u.User != nil || len(v.Data.URL) > 2000 {
		return contract.ParameterQRCode{}, fmt.Errorf("wxpusher QR URL invalid")
	}
	return contract.ParameterQRCode{Code: v.Data.Code, URL: v.Data.URL}, nil
}
func (s *Sender) ScanUID(ctx context.Context, code string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.providerURL("/api/fun/scan-qrcode-uid")+"?code="+url.QueryEscape(code), nil)
	if err != nil {
		return "", err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wxpusher scan query failed")
	}
	defer resp.Body.Close()
	var v struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&v) != nil || resp.StatusCode != 200 || v.Code != 1000 {
		return "", fmt.Errorf("wxpusher scan response invalid")
	}
	// The official API returns a UID string; accept the structured UID variant too.
	uid := ""
	if len(v.Data) > 0 && string(v.Data) != "null" {
		if json.Unmarshal(v.Data, &uid) != nil {
			var data struct {
				UID string `json:"uid"`
			}
			if json.Unmarshal(v.Data, &data) != nil {
				return "", fmt.Errorf("wxpusher scan data invalid")
			}
			uid = data.UID
		}
	}
	if uid != "" && (!strings.HasPrefix(uid, "UID_") || len(uid) < 8 || len(uid) > 80) {
		return "", fmt.Errorf("wxpusher UID invalid")
	}
	return uid, nil
}
