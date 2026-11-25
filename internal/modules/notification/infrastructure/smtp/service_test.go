package smtp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/i18n"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestBuildOrderStatusContent(t *testing.T) {
	tests := []struct {
		name                string
		locale              string
		status              string
		payload             string
		wantSubjectContains []string
		wantBodyContains    []string
	}{
		{
			name:   "paid_zh",
			locale: i18n.LocaleZH,
			status: "paid",
			wantSubjectContains: []string{
				"订单状态更新",
				"已支付",
			},
			wantBodyContains: []string{
				"已收到您的付款",
				"订单号：DJ-PAID",
			},
		},
		{
			name:    "delivered_with_payload_tw",
			locale:  i18n.LocaleTW,
			status:  "delivered",
			payload: "CODE-A\nCODE-B",
			wantSubjectContains: []string{
				"訂單狀態更新",
				"已交付",
			},
			wantBodyContains: []string{
				"交付內容",
				"CODE-A",
			},
		},
		{
			name:    "delivered_no_payload_en",
			locale:  i18n.LocaleEN,
			status:  "delivered",
			payload: "",
			wantSubjectContains: []string{
				"Order status updated",
				"Delivered",
			},
			wantBodyContains: []string{
				"Delivery completed",
				"Order No: DJ-DELIVER",
			},
		},
		{
			name:    "completed_with_payload_zh",
			locale:  i18n.LocaleZH,
			status:  "completed",
			payload: "AUTO-CODE-001",
			wantSubjectContains: []string{
				"订单状态更新",
				"已完成",
			},
			wantBodyContains: []string{
				"交付内容",
				"AUTO-CODE-001",
			},
		},
		{
			name:   "refunded_zh",
			locale: i18n.LocaleZH,
			status: "refunded",
			wantSubjectContains: []string{
				"订单状态更新",
				"已退款",
			},
			wantBodyContains: []string{
				"退款金额：8.80 USD",
				"退款原因：manual refund",
				"示例站点 的网址：https://example.com",
			},
		},
		{
			name:   "partially_refunded_en",
			locale: i18n.LocaleEN,
			status: "partially_refunded",
			wantSubjectContains: []string{
				"Order status updated",
				"Partially refunded",
			},
			wantBodyContains: []string{
				"Refund Amount: 8.80 USD",
				"Reason for refund: manual refund",
				"Example Site's Site URL: https://example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := notificationcontract.OrderStatusEmailInput{
				OrderNo:         pickOrderNo(tt.status),
				Status:          tt.status,
				Amount:          money.FromDecimal(decimal.NewFromFloat(19.8)),
				RefundAmount:    money.FromDecimal(decimal.NewFromFloat(8.8)),
				RefundReason:    "manual refund",
				Currency:        "USD",
				SiteName:        "Example Site",
				SiteURL:         "https://example.com",
				FulfillmentInfo: tt.payload,
			}
			if tt.locale == i18n.LocaleZH {
				input.SiteName = "示例站点"
			}
			subject, body := buildOrderStatusContent(input, tt.locale)
			for _, expected := range tt.wantSubjectContains {
				if !strings.Contains(subject, expected) {
					t.Fatalf("subject missing %q: %s", expected, subject)
				}
			}
			for _, expected := range tt.wantBodyContains {
				if !strings.Contains(body, expected) {
					t.Fatalf("body missing %q: %s", expected, body)
				}
			}
			if strings.Contains(body, "%!") {
				t.Fatalf("body contains fmt placeholder error marker: %s", body)
			}
		})
	}
}

func pickOrderNo(status string) string {
	switch status {
	case "paid":
		return "DJ-PAID"
	case "refunded":
		return "DJ-REFUND"
	case "partially_refunded":
		return "DJ-PART-REFUND"
	default:
		return "DJ-DELIVER"
	}
}

func TestIsEmailRecipientRejected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "smtp_550_no_such_recipient",
			err:  errors.New("550 No such recipient here"),
			want: true,
		},
		{
			name: "smtp_user_unknown",
			err:  errors.New("SMTP 5.1.1 user unknown"),
			want: true,
		},
		{
			name: "smtp_550_mailbox_unavailable",
			err:  errors.New("550 mailbox unavailable"),
			want: true,
		},
		{
			name: "network_timeout",
			err:  errors.New("dial tcp timeout"),
			want: false,
		},
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmailRecipientRejected(tt.err); got != tt.want {
				t.Fatalf("isEmailRecipientRejected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeEmailSendError(t *testing.T) {
	rejected := errors.New("550 No such recipient here")
	if got := normalizeEmailSendError(rejected); !errors.Is(got, notificationcontract.ErrEmailRecipientRejected) {
		t.Fatalf("normalizeEmailSendError() expected ErrEmailRecipientRejected, got %v", got)
	}

	networkErr := errors.New("dial tcp timeout")
	if got := normalizeEmailSendError(networkErr); !errors.Is(got, networkErr) {
		t.Fatalf("normalizeEmailSendError() should keep original error, got %v", got)
	}

	if got := normalizeEmailSendError(nil); got != nil {
		t.Fatalf("normalizeEmailSendError(nil) should be nil, got %v", got)
	}
}

func TestSendEmailSkipsTelegramPlaceholder(t *testing.T) {
	service := &Service{}
	if err := service.SendCustomEmail("telegram_6059928735@login.local", "subject", "body"); err != nil {
		t.Fatalf("send email should skip telegram placeholder email, got %v", err)
	}
}

func TestBuildOrderStatusContentFromTemplateIncludesSiteBrand(t *testing.T) {
	tmpl := settingsmessaging.DefaultOrderEmailTemplateSetting()
	tmpl.Templates.Paid.ZHCN.Subject = "订单通知 {{site_name}}"
	tmpl.Templates.Paid.ZHCN.Body = "订单号：{{order_no}}\n站点：{{site_name}} {{site_url}}"

	input := notificationcontract.OrderStatusEmailInput{
		OrderNo:         "DJ-SITE-001",
		Status:          "paid",
		Amount:          money.FromDecimal(decimal.NewFromInt(10)),
		Currency:        "CNY",
		SiteName:        " 示例站点 ",
		SiteURL:         " https://example.com/shop ",
		IsGuest:         false,
		FulfillmentInfo: "",
	}

	subject, body := buildOrderStatusContentFromTemplate(input, i18n.LocaleZH, tmpl)

	if !strings.Contains(subject, "示例站点") {
		t.Fatalf("subject should contain site_name, got: %s", subject)
	}
	if !strings.Contains(body, "站点：示例站点 https://example.com/shop") {
		t.Fatalf("body should contain site_name and site_url, got: %s", body)
	}
}

func TestBuildVerifyCodeContentUsesResolvedWhiteLabelBrand(t *testing.T) {
	subject, body := buildVerifyCodeContent("123456", "register", i18n.LocaleZH, mailbrand.Brand{
		SiteName: "白标商店",
		SiteURL:  "https://shop.example.test",
	})

	if !strings.Contains(subject, "白标商店") {
		t.Fatalf("subject should contain white-label site name, got: %s", subject)
	}
	if !strings.Contains(body, "白标商店") || !strings.Contains(body, "https://shop.example.test") {
		t.Fatalf("body should contain white-label brand only, got: %s", body)
	}
	if strings.Contains(body, "Dujiao-Next") || strings.Contains(body, "main.example.test") {
		t.Fatalf("body leaked main-site brand: %s", body)
	}
}

func TestBuildEmailMessageSupportsPerMessageFromNameAndReplyTo(t *testing.T) {
	from := buildFromAddress("mailer@service.example", "白标商店")
	parsedFrom, err := mail.ParseAddress(from)
	if err != nil {
		t.Fatalf("parse from address failed: %v", err)
	}
	if parsedFrom.Name != "白标商店" {
		t.Fatalf("from display name was encoded incorrectly: %q", parsedFrom.Name)
	}
	raw, err := buildEmailMessage(from, "buyer@example.com", emailMessage{
		Subject: "subject", Text: "body", Brand: mailbrand.Brand{ReplyTo: "support@shop.example.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	message := string(raw)

	if !strings.Contains(message, "From: ") || !strings.Contains(message, "<mailer@service.example>") {
		t.Fatalf("message should retain the authenticated shared sender address: %s", message)
	}
	if !strings.Contains(message, "Reply-To: support@shop.example.test\r\n") {
		t.Fatalf("message should contain tenant reply-to: %s", message)
	}
}

func TestPickSMTPAuthMechanism(t *testing.T) {
	tests := []struct {
		name       string
		advertised string
		want       string
	}{
		{name: "prefer_login_when_both_exist", advertised: "PLAIN LOGIN XOAUTH2", want: smtpAuthMechanismLogin},
		{name: "plain_only", advertised: "PLAIN", want: smtpAuthMechanismPlain},
		{name: "case_and_space", advertised: "  login   xoauth2 ", want: smtpAuthMechanismLogin},
		{name: "unsupported", advertised: "CRAM-MD5", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickSMTPAuthMechanism(tt.advertised); got != tt.want {
				t.Fatalf("pickSMTPAuthMechanism() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoginAuth(t *testing.T) {
	auth := newLoginAuth("user@example.com", "pwd", "smtp.office365.com")
	login, ok := auth.(*loginAuth)
	if !ok {
		t.Fatal("newLoginAuth() should return *loginAuth")
	}

	if _, _, err := login.Start(&smtp.ServerInfo{Name: "smtp.office365.com", TLS: true}); err != nil {
		t.Fatalf("Start() returned unexpected error: %v", err)
	}

	resp, err := login.Next([]byte("Username:"), true)
	if err != nil || string(resp) != "user@example.com" {
		t.Fatalf("Next(username) = %q, %v", string(resp), err)
	}

	resp, err = login.Next([]byte("Password:"), true)
	if err != nil || string(resp) != "pwd" {
		t.Fatalf("Next(password) = %q, %v", string(resp), err)
	}

	resp, err = login.Next(nil, false)
	if err != nil || resp != nil {
		t.Fatalf("Next(done) = %v, %v; want nil, nil", resp, err)
	}
}

func TestLoginAuthRejectsInsecureRemoteConnection(t *testing.T) {
	auth := newLoginAuth("user@example.com", "pwd", "smtp.office365.com")
	login := auth.(*loginAuth)

	if _, _, err := login.Start(&smtp.ServerInfo{Name: "smtp.office365.com", TLS: false}); err == nil {
		t.Fatal("Start() should reject insecure remote connection")
	}
}

func TestSMTPServerAuthExtensions(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("TEST_SMTP_HOST"))
	if host == "" {
		t.Skip("set TEST_SMTP_HOST to check SMTP server AUTH extensions")
	}

	port := 587
	if rawPort := strings.TrimSpace(os.Getenv("TEST_SMTP_PORT")); rawPort != "" {
		if parsed, err := strconv.Atoi(rawPort); err == nil && parsed > 0 {
			port = parsed
		}
	}

	useStartTLS := true
	if rawStartTLS := strings.TrimSpace(os.Getenv("TEST_SMTP_USE_STARTTLS")); rawStartTLS != "" {
		if parsed, err := strconv.ParseBool(rawStartTLS); err == nil {
			useStartTLS = parsed
		}
	}

	insecureSkipVerify := false
	if rawInsecure := strings.TrimSpace(os.Getenv("TEST_SMTP_INSECURE_SKIP_VERIFY")); rawInsecure != "" {
		if parsed, err := strconv.ParseBool(rawInsecure); err == nil {
			insecureSkipVerify = parsed
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, closeFn, err := newSMTPTestClient(addr, host, port, useStartTLS, insecureSkipVerify)
	if err != nil {
		t.Fatalf("connect smtp server failed: %v", err)
	}
	t.Cleanup(closeFn)

	ok, authLine := client.Extension("AUTH")
	if !ok {
		t.Logf("smtp server %s does not advertise AUTH", addr)
		return
	}

	mechanisms := strings.Fields(strings.ToUpper(strings.TrimSpace(authLine)))
	if len(mechanisms) == 0 {
		t.Fatalf("smtp AUTH extension is empty: %q", authLine)
	}

	t.Logf("smtp server %s supports AUTH: %s", addr, strings.Join(mechanisms, ", "))
}

func newSMTPTestClient(addr, host string, port int, useStartTLS, insecureSkipVerify bool) (*smtp.Client, func(), error) {
	if port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, InsecureSkipVerify: insecureSkipVerify})
		if err != nil {
			return nil, nil, err
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		return client, func() {
			if err := client.Quit(); err != nil {
				_ = client.Close()
			}
		}, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, nil, err
	}
	if useStartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: host, InsecureSkipVerify: insecureSkipVerify}); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}
	return client, func() {
		if err := client.Quit(); err != nil {
			_ = client.Close()
		}
	}, nil
}

// 仅是针对 Office365 SMTP 服务器的集成测试，确保 Service 能够成功发送邮件并正确处理服务器的响应。
// 需要在环境变量中设置 TEST_OFFICE365_SEND=1 和有效的 TEST_OFFICE365_PASSWORD 来运行此测试。
func TestEmailServiceSendOffice365Integration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_OFFICE365_SEND")) != "1" {
		t.Skip("set TEST_OFFICE365_SEND=1 to send a real email via Office365")
	}

	username := strings.TrimSpace(os.Getenv("TEST_OFFICE365_USERNAME"))
	if username == "" {
		t.Skip("set TEST_OFFICE365_USERNAME")
	}

	password := strings.TrimSpace(os.Getenv("TEST_OFFICE365_PASSWORD"))
	if password == "" {
		t.Skip("set TEST_OFFICE365_PASSWORD")
	}

	to := strings.TrimSpace(os.Getenv("TEST_OFFICE365_TO"))
	if to == "" {
		t.Skip("set TEST_OFFICE365_TO")
	}

	from := strings.TrimSpace(os.Getenv("TEST_OFFICE365_FROM"))
	if from == "" {
		from = username
	}

	svc := New(&config.EmailConfig{
		Enabled:  true,
		Host:     "smtp.office365.com",
		Port:     587,
		Username: username,
		Password: password,
		From:     from,
		FromName: "Dujiao Next",
		UseTLS:   true,
		UseSSL:   false,
	}, nil)

	if err := svc.SendCustomEmail(to, "Office365 SMTP integration test", "This is a real email sent by Service integration test."); err != nil {
		t.Fatalf("SendCustomEmail() failed: %v", err)
	}

	t.Logf("email sent via smtp.office365.com:587 to %s", to)
}

type fakeSMTPSessionCloser struct {
	quitErr  error
	closeErr error
	quitCnt  int
	closeCnt int
}

func (f *fakeSMTPSessionCloser) Quit() error {
	f.quitCnt++
	return f.quitErr
}

func (f *fakeSMTPSessionCloser) Close() error {
	f.closeCnt++
	return f.closeErr
}

func TestQuitSMTPClient(t *testing.T) {
	t.Run("quit_success_no_close", func(t *testing.T) {
		fake := &fakeSMTPSessionCloser{}
		if err := quitSMTPClient(fake, "smtp.office365.com", "smtp.office365.com:587"); err != nil {
			t.Fatalf("quitSMTPClient() returned error: %v", err)
		}
		if fake.quitCnt != 1 {
			t.Fatalf("Quit() called %d times, want 1", fake.quitCnt)
		}
		if fake.closeCnt != 0 {
			t.Fatalf("Close() called %d times, want 0", fake.closeCnt)
		}
	})

	t.Run("quit_failed_fallback_close", func(t *testing.T) {
		quitErr := errors.New("421 service not available")
		fake := &fakeSMTPSessionCloser{quitErr: quitErr}
		if err := quitSMTPClient(fake, "smtp.office365.com", "smtp.office365.com:587"); !errors.Is(err, quitErr) {
			t.Fatalf("quitSMTPClient() error = %v, want %v", err, quitErr)
		}
		if fake.quitCnt != 1 {
			t.Fatalf("Quit() called %d times, want 1", fake.quitCnt)
		}
		if fake.closeCnt != 1 {
			t.Fatalf("Close() called %d times, want 1", fake.closeCnt)
		}
	})
}

func TestIsSMTPAlreadyClosedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "closed_network_connection", err: errors.New("use of closed network connection"), want: true},
		{name: "connection_is_closed", err: errors.New("connection is closed"), want: true},
		{name: "other_error", err: errors.New("dial tcp timeout"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSMTPAlreadyClosedError(tt.err); got != tt.want {
				t.Fatalf("isSMTPAlreadyClosedError() = %v, want %v", got, tt.want)
			}
		})
	}
}
