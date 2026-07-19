package service

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/telegramidentity"
)

// writeStandardHeaders 写入 RFC 5322 要求的通用邮件头（Date、Message-ID、From、To、Subject、MIME-Version）。
func writeStandardHeaders(buf *bytes.Buffer, from, to, subject string) {
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString(fmt.Sprintf("Message-ID: %s\r\n", generateMessageID(from)))
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject)))
	buf.WriteString("MIME-Version: 1.0\r\n")
}

// generateMessageID 生成 RFC 5322 兼容的 Message-ID，域名取自 From 地址，失败则回退到 localhost。
func generateMessageID(from string) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	domain := "localhost"
	if addr, err := mail.ParseAddress(from); err == nil {
		if i := strings.LastIndex(addr.Address, "@"); i >= 0 && i < len(addr.Address)-1 {
			domain = addr.Address[i+1:]
		}
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b[:]), domain)
}

// EmailService 邮件发送服务
type EmailService struct {
	cfg *config.EmailConfig
}

// NewEmailService 创建邮件服务
func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return &EmailService{cfg: cfg}
}

// SetConfig 更新运行时邮件配置
func (s *EmailService) SetConfig(cfg *config.EmailConfig) {
	if cfg == nil {
		return
	}
	s.cfg = cfg
}

// SendVerifyCode 发送邮箱验证码
func (s *EmailService) SendVerifyCode(toEmail, code, purpose, locale string) error {
	subject, body := buildVerifyCodeContent(code, purpose, locale)
	return s.sendTextEmail(toEmail, subject, body)
}

// OrderStatusEmailInput 订单状态邮件输入
type OrderStatusEmailInput struct {
	OrderNo           string
	Status            string
	Amount            models.Money
	RefundAmount      models.Money
	RefundReason      string
	Currency          string
	SiteName          string
	SiteURL           string
	FulfillmentInfo   string
	Instructions      string // 交付使用说明（纯文本，已去 HTML）
	IsGuest           bool
	AttachmentName    string // 非空时表示交付内容以附件形式发送
	AttachmentContent string // 附件内容
}

// SendOrderStatusEmail 发送订单状态通知
func (s *EmailService) SendOrderStatusEmail(toEmail string, input OrderStatusEmailInput, locale string) error {
	subject, body := buildOrderStatusContent(input, locale)
	if input.AttachmentName != "" && input.AttachmentContent != "" {
		return s.sendEmailWithAttachment(toEmail, subject, body, input.AttachmentName, input.AttachmentContent)
	}
	return s.sendTextEmail(toEmail, subject, body)
}

// SendOrderStatusEmailWithTemplate 使用可配置模板发送订单状态通知
func (s *EmailService) SendOrderStatusEmailWithTemplate(toEmail string, input OrderStatusEmailInput, locale string, tmplSetting *OrderEmailTemplateSetting) error {
	if tmplSetting == nil {
		return s.SendOrderStatusEmail(toEmail, input, locale)
	}
	subject, body := buildOrderStatusContentFromTemplate(input, locale, *tmplSetting)
	if input.AttachmentName != "" && input.AttachmentContent != "" {
		return s.sendEmailWithAttachment(toEmail, subject, body, input.AttachmentName, input.AttachmentContent)
	}
	return s.sendTextEmail(toEmail, subject, body)
}

func buildOrderStatusContentFromTemplate(input OrderStatusEmailInput, locale string, tmplSetting OrderEmailTemplateSetting) (string, string) {
	normalized := normalizeLocale(locale)

	// 根据订单状态选择场景模板
	var sceneTmpl OrderEmailSceneTemplate
	status := strings.ToLower(strings.TrimSpace(input.Status))
	switch status {
	case constants.OrderStatusPaid:
		sceneTmpl = tmplSetting.Templates.Paid
	case constants.OrderStatusDelivered, constants.OrderStatusFulfilled, constants.OrderStatusCompleted:
		if strings.TrimSpace(input.FulfillmentInfo) != "" {
			sceneTmpl = tmplSetting.Templates.DeliveredWithContent
		} else {
			sceneTmpl = tmplSetting.Templates.Delivered
		}
	case constants.OrderStatusCanceled:
		sceneTmpl = tmplSetting.Templates.Canceled
	case constants.OrderStatusRefunded:
		sceneTmpl = tmplSetting.Templates.Refunded
	case constants.OrderStatusPartiallyRefunded:
		sceneTmpl = tmplSetting.Templates.PartiallyRefunded
	default:
		sceneTmpl = tmplSetting.Templates.Default
	}

	localeTmpl := ResolveOrderEmailLocaleTemplate(sceneTmpl, normalized)

	// 翻译状态标签
	statusKey := "order.status." + status
	statusLabel := i18n.T(normalized, statusKey)
	if statusLabel == statusKey {
		statusLabel = input.Status
	}

	variables := map[string]interface{}{
		"order_no":         input.OrderNo,
		"status":           statusLabel,
		"amount":           input.Amount.String(),
		"refund_amount":    "",
		"refund_reason":    "",
		"currency":         strings.TrimSpace(input.Currency),
		"site_name":        strings.TrimSpace(input.SiteName),
		"site_url":         strings.TrimSpace(input.SiteURL),
		"fulfillment_info": strings.TrimSpace(input.FulfillmentInfo),
		"instructions":     strings.TrimSpace(input.Instructions),
	}
	if status == constants.OrderStatusRefunded || status == constants.OrderStatusPartiallyRefunded {
		variables["refund_amount"] = input.RefundAmount.String()
		variables["refund_reason"] = strings.TrimSpace(input.RefundReason)
	}

	subject := renderTemplate(localeTmpl.Subject, variables)
	body := renderTemplate(localeTmpl.Body, variables)

	// 存量兼容：历史自定义模板未引用 {{instructions}} 时自动在末尾追加使用说明，避免因占位符缺失导致说明丢失。
	// 只在交付含内容场景生效（此时 input.Instructions 才会被填充）。
	if strings.TrimSpace(input.Instructions) != "" && !strings.Contains(localeTmpl.Body, "{{instructions}}") {
		body = strings.TrimRight(body, "\n") + "\n\n" + strings.TrimSpace(input.Instructions)
	}

	// 交付内容以附件形式发送时追加提示
	if input.AttachmentName != "" {
		tip := strings.TrimSpace(ResolveOrderEmailFulfillmentAttachmentTip(tmplSetting.FulfillmentAttachmentTip, normalized))
		if tip != "" {
			body = body + "\n\n" + tip
		}
	}

	// 游客订单追加提示
	if input.IsGuest {
		tip := strings.TrimSpace(ResolveOrderEmailGuestTip(tmplSetting.GuestTip, normalized))
		if tip != "" {
			body = body + "\n\n" + tip
		}
	}

	return subject, body
}

// SendCustomEmail 发送测试邮件或自定义邮件
func (s *EmailService) SendCustomEmail(toEmail, subject, body string) error {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "SMTP 配置测试邮件"
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = "这是一封来自 Dujiao-Next 的 SMTP 测试邮件，说明当前配置可正常发送。"
	}
	return s.sendTextEmail(toEmail, subject, body)
}

func (s *EmailService) sendTextEmail(toEmail, subject, body string) error {
	if telegramidentity.IsPlaceholderEmail(toEmail) {
		return nil
	}
	from, addr, err := s.prepareSMTPEnvelope(toEmail)
	if err != nil {
		return err
	}
	msg := buildEmailMessage(from, toEmail, subject, body)
	return s.sendSMTPMessage(addr, toEmail, []byte(msg))
}

func (s *EmailService) sendEmailWithAttachment(toEmail, subject, body, attachName, attachContent string) error {
	if telegramidentity.IsPlaceholderEmail(toEmail) {
		return nil
	}
	from, addr, err := s.prepareSMTPEnvelope(toEmail)
	if err != nil {
		return err
	}
	msg := buildEmailMessageWithAttachment(from, toEmail, subject, body, attachName, attachContent)
	return s.sendSMTPMessage(addr, toEmail, []byte(msg))
}

// prepareSMTPEnvelope 校验配置与收件人，并返回发件地址与 SMTP 服务器地址。
func (s *EmailService) prepareSMTPEnvelope(toEmail string) (string, string, error) {
	if s.cfg == nil || !s.cfg.Enabled {
		return "", "", ErrEmailServiceDisabled
	}
	if s.cfg.Host == "" || s.cfg.Port == 0 || s.cfg.From == "" {
		return "", "", ErrEmailServiceNotConfigured
	}
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return "", "", ErrInvalidEmail
	}
	from := buildFromAddress(s.cfg.From, s.cfg.FromName)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return from, addr, nil
}

// sendSMTPMessage 根据配置选择 SSL/STARTTLS/明文通道发送邮件。
func (s *EmailService) sendSMTPMessage(addr, toEmail string, msg []byte) error {
	recipients := []string{toEmail}
	if s.cfg.UseSSL {
		return normalizeEmailSendError(sendMailWithSSL(addr, s.cfg.Host, s.cfg.From, recipients, msg, s.cfg.Username, s.cfg.Password))
	}
	if s.cfg.UseTLS {
		return normalizeEmailSendError(sendMailWithStartTLS(addr, s.cfg.Host, s.cfg.From, recipients, msg, s.cfg.Username, s.cfg.Password))
	}
	return normalizeEmailSendError(sendMailPlain(addr, s.cfg.Host, s.cfg.From, recipients, msg, s.cfg.Username, s.cfg.Password))
}

func buildEmailMessageWithAttachment(from, to, subject, body, attachName, attachContent string) string {
	boundary := "----=_DujiaoNextBoundary_" + fmt.Sprintf("%d", len(body)+len(attachContent))

	var buf bytes.Buffer
	writeStandardHeaders(&buf, from, to, subject)
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	buf.WriteString("\r\n")

	// 正文部分
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(body)))
	buf.WriteString("\r\n")

	// 附件部分
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", mime.QEncoding.Encode("UTF-8", attachName)))
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(attachContent)))
	buf.WriteString("\r\n")

	// 结束边界
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.String()
}

func buildVerifyCodeContent(code, purpose, locale string) (string, string) {
	normalized := normalizeLocale(locale)
	purposeKey := strings.ToLower(strings.TrimSpace(purpose))
	switch normalized {
	case i18n.LocaleTW:
		subject := "郵箱驗證碼"
		purposeText := "郵箱驗證"
		switch purposeKey {
		case constants.VerifyPurposeRegister:
			subject = "註冊驗證碼"
			purposeText = "註冊"
		case constants.VerifyPurposeReset:
			subject = "重置密碼驗證碼"
			purposeText = "重置密碼"
		case constants.VerifyPurposeTelegramBind:
			subject = "Telegram 綁定驗證碼"
			purposeText = "綁定 Telegram"
		case constants.VerifyPurposeChangeEmailOld, constants.VerifyPurposeChangeEmailNew:
			subject = "更換郵箱驗證碼"
			purposeText = "更換郵箱"
		}
		body := fmt.Sprintf("您的驗證碼是：%s\n\n該驗證碼用於 %s，請勿洩露。", code, purposeText)
		return subject, body
	case i18n.LocaleEN:
		subject := "Email Verification Code"
		purposeText := "email verification"
		switch purposeKey {
		case constants.VerifyPurposeRegister:
			subject = "Registration Code"
			purposeText = "registration"
		case constants.VerifyPurposeReset:
			subject = "Password Reset Code"
			purposeText = "password reset"
		case constants.VerifyPurposeTelegramBind:
			subject = "Telegram Binding Code"
			purposeText = "binding Telegram"
		case constants.VerifyPurposeChangeEmailOld, constants.VerifyPurposeChangeEmailNew:
			subject = "Change Email Code"
			purposeText = "change email"
		}
		body := fmt.Sprintf("Your verification code is: %s\n\nThis code is for %s. Do not share it.", code, purposeText)
		return subject, body
	default:
		subject := "邮箱验证码"
		purposeText := "邮箱验证"
		switch purposeKey {
		case constants.VerifyPurposeRegister:
			subject = "注册验证码"
			purposeText = "注册"
		case constants.VerifyPurposeReset:
			subject = "重置密码验证码"
			purposeText = "重置密码"
		case constants.VerifyPurposeTelegramBind:
			subject = "Telegram 绑定验证码"
			purposeText = "绑定 Telegram"
		case constants.VerifyPurposeChangeEmailOld, constants.VerifyPurposeChangeEmailNew:
			subject = "更换邮箱验证码"
			purposeText = "更换邮箱"
		}
		body := fmt.Sprintf("您的验证码是：%s\n\n该验证码用于 %s，请勿泄露。", code, purposeText)
		return subject, body
	}
}

func buildOrderStatusContent(input OrderStatusEmailInput, locale string) (string, string) {
	normalized := normalizeLocale(locale)
	statusKey := "order.status." + strings.ToLower(strings.TrimSpace(input.Status))
	statusLabel := i18n.T(normalized, statusKey)
	if statusLabel == statusKey {
		statusLabel = input.Status
	}
	amount := input.Amount.String()
	refundAmount := input.RefundAmount.String()
	refundReason := strings.TrimSpace(input.RefundReason)
	currency := strings.TrimSpace(input.Currency)
	siteName := strings.TrimSpace(input.SiteName)
	siteURL := strings.TrimSpace(input.SiteURL)
	subject := i18n.Sprintf(normalized, "email.order_status.subject", statusLabel)
	payload := strings.TrimSpace(input.FulfillmentInfo)
	status := strings.ToLower(strings.TrimSpace(input.Status))
	switch status {
	case constants.OrderStatusDelivered, constants.OrderStatusFulfilled, constants.OrderStatusCompleted:
		if payload != "" {
			body := i18n.Sprintf(normalized, "email.order_status.body_delivered", input.OrderNo, statusLabel, amount, currency, payload, siteName, siteURL)
			return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
		}
		body := i18n.Sprintf(normalized, "email.order_status.body_delivered_simple", input.OrderNo, statusLabel, amount, currency, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	case constants.OrderStatusPaid:
		body := i18n.Sprintf(normalized, "email.order_status.body_paid", input.OrderNo, statusLabel, amount, currency, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	case constants.OrderStatusCanceled:
		body := i18n.Sprintf(normalized, "email.order_status.body_canceled", input.OrderNo, statusLabel, amount, currency, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	case constants.OrderStatusRefunded:
		body := i18n.Sprintf(normalized, "email.order_status.body_refunded", input.OrderNo, statusLabel, refundAmount, currency, refundReason, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	case constants.OrderStatusPartiallyRefunded:
		body := i18n.Sprintf(normalized, "email.order_status.body_partially_refunded", input.OrderNo, statusLabel, refundAmount, currency, refundReason, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	default:
		body := i18n.Sprintf(normalized, "email.order_status.body", input.OrderNo, statusLabel, amount, currency, siteName, siteURL)
		return subject, appendGuestTip(normalized, input, appendFulfillmentAttachmentTip(normalized, input, body))
	}
}

func appendFulfillmentAttachmentTip(locale string, input OrderStatusEmailInput, body string) string {
	if input.AttachmentName == "" {
		return body
	}
	tipKey := "email.order_status.fulfillment_attachment_tip"
	tip := i18n.T(locale, tipKey)
	if tip == tipKey {
		return body
	}
	return body + "\n\n" + tip
}

func appendGuestTip(locale string, input OrderStatusEmailInput, body string) string {
	if !input.IsGuest {
		return body
	}
	tipKey := "email.order_status.guest_tip"
	tip := i18n.T(locale, tipKey)
	if tip == tipKey {
		return body
	}
	return body + "\n\n" + tip
}

func normalizeLocale(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	switch {
	case strings.HasPrefix(l, "zh-tw"), strings.HasPrefix(l, "zh-hk"), strings.HasPrefix(l, "zh-mo"):
		return i18n.LocaleTW
	case strings.HasPrefix(l, "en"):
		return i18n.LocaleEN
	default:
		return i18n.LocaleZH
	}
}

func buildFromAddress(from, name string) string {
	if strings.TrimSpace(name) == "" {
		return from
	}
	encoded := mime.QEncoding.Encode("UTF-8", name)
	return (&mail.Address{Name: encoded, Address: from}).String()
}

func buildEmailMessage(from, to, subject, body string) string {
	var buf bytes.Buffer
	writeStandardHeaders(&buf, from, to, subject)
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.String()
}

func sendMailWithSSL(addr, host, from string, to []string, msg []byte, username, password string) (err error) {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil && !isSMTPAlreadyClosedError(closeErr) {
			logger.Debugw("smtp_tls_conn_close_failed", "host", host, "addr", addr, "error", closeErr)
		}
		return err
	}
	defer closeSMTPClientOnError(client, &err, host, addr)

	if err := authenticateSMTPClient(client, host, username, password); err != nil {
		return err
	}

	err = sendSMTPData(client, host, addr, from, to, msg)
	return err
}

func sendMailWithStartTLS(addr, host, from string, to []string, msg []byte, username, password string) (err error) {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer closeSMTPClientOnError(client, &err, host, addr)

	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return err
	}

	if err := authenticateSMTPClient(client, host, username, password); err != nil {
		return err
	}

	err = sendSMTPData(client, host, addr, from, to, msg)
	return err
}

func sendMailPlain(addr, host, from string, to []string, msg []byte, username, password string) (err error) {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer closeSMTPClientOnError(client, &err, host, addr)

	if err := authenticateSMTPClient(client, host, username, password); err != nil {
		return err
	}

	err = sendSMTPData(client, host, addr, from, to, msg)
	return err
}

const (
	smtpAuthMechanismPlain = "PLAIN"
	smtpAuthMechanismLogin = "LOGIN"
	// 还有 XOAUTH2 等机制，当前实现暂不处理。
)

// authenticateSMTPClient 根据服务端 AUTH 能力选择并执行认证。
// EmailService 认证策略：优先 LOGIN，回退 PLAIN。
// 对 smtp.office365.com 的 SMTP Basic/LOGIN 场景，通常需要开启 MFA 并使用应用密码。
func authenticateSMTPClient(client *smtp.Client, host, username, password string) error {
	if client == nil {
		return nil
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" && password == "" {
		return nil
	}

	ok, advertised := client.Extension("AUTH")
	if !ok {
		return nil
	}

	switch pickSMTPAuthMechanism(advertised) {
	case smtpAuthMechanismLogin:
		return client.Auth(newLoginAuth(username, password, host))
	case smtpAuthMechanismPlain:
		return client.Auth(smtp.PlainAuth("", username, password, host))
	default:
		return fmt.Errorf("smtp auth mechanism not supported (server AUTH=%q)", advertised)
	}
}

// pickSMTPAuthMechanism 根据服务端 AUTH 能力选择机制，优先 LOGIN，再回退 PLAIN。
func pickSMTPAuthMechanism(advertised string) string {
	if hasSMTPAuthMechanism(advertised, smtpAuthMechanismLogin) {
		return smtpAuthMechanismLogin
	}
	if hasSMTPAuthMechanism(advertised, smtpAuthMechanismPlain) {
		return smtpAuthMechanismPlain
	}
	return ""
}

// hasSMTPAuthMechanism 判断服务端 AUTH 扩展是否包含指定机制。
func hasSMTPAuthMechanism(advertised, mechanism string) bool {
	if strings.TrimSpace(mechanism) == "" {
		return false
	}
	tokens := strings.Fields(strings.ToUpper(strings.TrimSpace(advertised)))
	needle := strings.ToUpper(strings.TrimSpace(mechanism))
	for _, token := range tokens {
		if token == needle {
			return true
		}
	}
	return false
}

type loginAuth struct {
	username string
	password string
	host     string
	userSent bool
}

// newLoginAuth 构造 AUTH LOGIN 认证器。
func newLoginAuth(username, password, host string) smtp.Auth {
	return &loginAuth{username: username, password: password, host: host}
}

// Start 校验连接安全性并声明 LOGIN 机制。
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server == nil {
		return "", nil, fmt.Errorf("smtp server info is required")
	}
	if server.Name != a.host {
		return "", nil, fmt.Errorf("wrong host name")
	}
	if !server.TLS {
		return "", nil, fmt.Errorf("unencrypted connection")
	}
	a.userSent = false
	return smtpAuthMechanismLogin, nil, nil
}

// Next 按服务端 challenge 顺序回送用户名与密码。
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	challenge := strings.ToLower(strings.TrimSpace(string(fromServer)))
	if strings.Contains(challenge, "password") {
		return []byte(a.password), nil
	}
	if strings.Contains(challenge, "username") || strings.Contains(challenge, "user name") {
		a.userSent = true
		return []byte(a.username), nil
	}
	if !a.userSent {
		a.userSent = true
		return []byte(a.username), nil
	}
	return []byte(a.password), nil
}

type smtpSessionCloser interface {
	Quit() error
	Close() error
}

// sendSMTPData 发送 SMTP Envelope 与邮件正文。
func sendSMTPData(client *smtp.Client, host, addr, from string, to []string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return quitSMTPClient(client, host, addr)
}

// quitSMTPClient 优先执行 SMTP QUIT；仅在 QUIT 失败时补偿 Close 回收连接。
func quitSMTPClient(client smtpSessionCloser, host, addr string) error {
	if client == nil {
		return nil
	}
	if err := client.Quit(); err != nil {
		if closeErr := client.Close(); closeErr != nil && !isSMTPAlreadyClosedError(closeErr) {
			logger.Debugw("smtp_close_after_quit_failed", "host", host, "addr", addr, "error", closeErr)
		}
		return err
	}
	return nil
}

// closeSMTPClientOnError 仅在发送流程异常时兜底 Close，避免成功路径重复关闭噪音。
func closeSMTPClientOnError(client *smtp.Client, sendErr *error, host, addr string) {
	if client == nil || sendErr == nil || *sendErr == nil {
		return
	}
	if err := client.Close(); err != nil && !isSMTPAlreadyClosedError(err) {
		logger.Debugw("smtp_close_failed", "host", host, "addr", addr, "error", err)
	}
}

// isSMTPAlreadyClosedError 识别连接已关闭类错误，避免重复记录无效噪音。
func isSMTPAlreadyClosedError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	return strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "closed connection") ||
		strings.Contains(message, "connection is closed")
}

// normalizeEmailSendError 将可识别的收件人拒绝错误归一化为业务错误码。
func normalizeEmailSendError(err error) error {
	if err == nil {
		return nil
	}
	if isEmailRecipientRejected(err) {
		return ErrEmailRecipientRejected
	}
	return err
}

// isEmailRecipientRejected 识别常见 SMTP 收件人不存在/被拒绝错误。
func isEmailRecipientRejected(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	directKeywords := []string{
		"no such recipient",
		"no such user",
		"recipient not found",
		"recipient address rejected",
		"invalid recipient",
		"user unknown",
		"unknown user",
		"unknown mailbox",
		"mailbox unavailable",
	}
	for _, keyword := range directKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	if strings.Contains(message, "550") {
		hints := []string{"recipient", "user", "mailbox", "address", "rcpt"}
		for _, hint := range hints {
			if strings.Contains(message, hint) {
				return true
			}
		}
	}
	return false
}
