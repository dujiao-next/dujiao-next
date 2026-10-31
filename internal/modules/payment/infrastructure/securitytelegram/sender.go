package securitytelegram

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

var errSend = errors.New("security Telegram delivery failed")
var labels = map[string]string{
	"scanner_unhealthy": "网关扫描或历史来源接收异常",
	"control_missing":   "支付控制记录缺失", "automation_frozen": "U 支付自动处理已冻结", "candidate_conflicts": "候选证据冲突",
	"chain_manual": "链核验待人工处理", "rpc_retry": "RPC 不可用或双源不一致", "chain_backlog": "链核验或结算积压超过 5 分钟",
	"intent_stalled": "付款创建恢复超过 5 分钟", "effects_manual": "支付后续任务待人工处理", "effects_stalled": "支付后续任务积压超过 5 分钟",
}

type Sender struct {
	token, chatID string
	client        *http.Client
}

func New(token, chatID string) (*Sender, error) {
	if !regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]{20,}$`).MatchString(token) || !regexp.MustCompile(`^-?[1-9][0-9]{0,18}$`).MatchString(chatID) {
		return nil, errors.New("invalid security Telegram configuration")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 7 * time.Second, IdleConnTimeout: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || host != "api.telegram.org" || port != "443" {
			return nil, errSend
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, errSend
		}
		for _, ip := range ips {
			ip = ip.Unmap()
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				return nil, errSend
			}
		}
		for _, ip := range ips {
			conn, err := (&net.Dialer{Timeout: 7 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errSend
	}
	return &Sender{token: token, chatID: chatID, client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (s *Sender) Send(ctx context.Context, row pd.SecurityAlert) error {
	label, ok := labels[row.Kind]
	if !ok || row.Count < 0 {
		return errSend
	}
	status := "告警"
	if !row.Active {
		status = "恢复"
	}
	message := fmt.Sprintf("[商城 U 支付 %s] %s\n数量：%d\n记录版本：%d\n请在商城运维记录中核对；此消息不改变付款或订单状态。", status, label, row.Count, row.Version)
	body, _ := json.Marshal(map[string]interface{}{"chat_id": s.chatID, "text": message, "disable_web_page_preview": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+s.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errSend
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return errSend
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errSend
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return errSend
	}
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID json.Number `json:"id"`
			} `json:"chat"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.OK || result.Result.MessageID <= 0 || result.Result.Chat.ID.String() != s.chatID {
		return errSend
	}
	return nil
}
