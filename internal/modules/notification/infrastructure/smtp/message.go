package smtp

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

//go:embed email_layout.html
var emailLayoutHTML string

var brandedEmailTemplate = template.Must(template.New("email").Parse(emailLayoutHTML))

type emailMessage struct {
	Subject           string
	Text              string
	Brand             mailbrand.Brand
	Layout            emailLayout
	AttachmentName    string
	AttachmentContent string
}

type emailLayout struct {
	Locale   string
	Title    string
	Brand    string
	SiteURL  string
	SiteHost string
	Lines    []string
	Code     string
	Validity string
	Warning  string
	Footer   string
}

func textEmailLayout(title, text, locale string, brand mailbrand.Brand) emailLayout {
	layout := emailLayout{
		Title: title, Locale: normalizeLocale(locale),
		Brand: strings.TrimSpace(brand.SiteName),
	}
	if text != "" {
		canonical := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
		layout.Lines = strings.Split(canonical, "\n")
	}
	if parsed, err := url.Parse(strings.TrimSpace(brand.SiteURL)); err == nil &&
		(parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != "" && parsed.User == nil {
		layout.SiteURL = parsed.String()
		layout.SiteHost = parsed.Host
	}
	if layout.Brand == "" {
		layout.Brand = layout.SiteHost
	}
	if layout.Brand == "" {
		layout.Brand = strings.TrimSpace(brand.FromName)
	}
	return layout
}

func verificationEmailLayout(subject, code, purpose, locale string, minutes int, brand mailbrand.Brand) emailLayout {
	title := strings.TrimPrefix(subject, strings.TrimSpace(brand.SiteName)+" - ")
	layout := textEmailLayout(title, "", locale, brand)
	layout.Code = code
	isRegister := strings.EqualFold(strings.TrimSpace(purpose), constants.VerifyPurposeRegister)
	switch layout.Locale {
	case i18n.LocaleEN:
		instruction := "Enter this verification code to confirm your request."
		if isRegister {
			instruction = "Enter this code on the registration page to verify your email address."
		}
		layout.Lines = []string{instruction}
		layout.Validity = fmt.Sprintf("Valid for %d minutes.", minutes)
		layout.Warning = "Do not share this code."
		layout.Footer = "If you did not make this request, you can ignore this email."
	case i18n.LocaleTW:
		instruction := "請輸入下方驗證碼以確認本次操作。"
		if isRegister {
			instruction = "請將下方驗證碼填入註冊頁面，完成信箱驗證。"
		}
		layout.Lines = []string{instruction}
		layout.Validity = fmt.Sprintf("%d 分鐘內有效", minutes)
		layout.Warning = "，請勿向他人透露驗證碼。"
		layout.Footer = "如果你沒有發起這項操作，可以忽略這封郵件。"
	default:
		instruction := "请输入下方验证码以确认本次操作。"
		layout.Footer = "如果你没有发起这项操作，可以忽略这封邮件。"
		if isRegister {
			instruction = "请将下方验证码填入注册页面，完成邮箱验证。"
			layout.Footer = "如果你没有申请注册，可以忽略这封邮件。"
		}
		layout.Lines = []string{instruction}
		layout.Validity = fmt.Sprintf("%d 分钟内有效", minutes)
		layout.Warning = "，请勿向他人透露验证码。"
	}
	return layout
}

func buildEmailMessage(from, to string, message emailMessage) ([]byte, error) {
	var html bytes.Buffer
	if err := brandedEmailTemplate.Execute(&html, message.Layout); err != nil {
		return nil, fmt.Errorf("render email layout: %w", err)
	}

	var buf bytes.Buffer
	writeStandardHeaders(&buf, from, to, message.Subject, message.Brand.ReplyTo)
	root := multipart.NewWriter(&buf)
	if message.AttachmentName == "" || message.AttachmentContent == "" {
		fmt.Fprintf(&buf, "Content-Type: %s\r\n\r\n", multipartType("alternative", root.Boundary()))
		if err := writeAlternatives(root, message, html.Bytes()); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	fmt.Fprintf(&buf, "Content-Type: %s\r\n\r\n", multipartType("mixed", root.Boundary()))
	alternative, err := nestedMultipart(root, "alternative")
	if err != nil {
		return nil, err
	}
	if err := writeAlternatives(alternative, message, html.Bytes()); err != nil {
		return nil, err
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": message.AttachmentName})
	if err := writeEncodedPart(root, "text/plain; charset=UTF-8", disposition, []byte(message.AttachmentContent)); err != nil {
		return nil, err
	}
	if err := root.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAlternatives(alternative *multipart.Writer, message emailMessage, html []byte) error {
	if err := writeEncodedPart(alternative, "text/plain; charset=UTF-8", "", []byte(message.Text)); err != nil {
		return err
	}
	if err := writeEncodedPart(alternative, "text/html; charset=UTF-8", "", html); err != nil {
		return err
	}
	return alternative.Close()
}

func nestedMultipart(parent *multipart.Writer, subtype string) (*multipart.Writer, error) {
	boundary := rand.Text()
	part, err := parent.CreatePart(textproto.MIMEHeader{"Content-Type": {multipartType(subtype, boundary)}})
	if err != nil {
		return nil, err
	}
	child := multipart.NewWriter(part)
	return child, child.SetBoundary(boundary)
}

func multipartType(subtype, boundary string) string {
	params := map[string]string{"boundary": boundary}
	return mime.FormatMediaType("multipart/"+subtype, params)
}

func writeEncodedPart(w *multipart.Writer, contentType, disposition string, data []byte) error {
	headers := textproto.MIMEHeader{
		"Content-Type":              {contentType},
		"Content-Transfer-Encoding": {"base64"},
	}
	if disposition != "" {
		headers.Set("Content-Disposition", disposition)
	}
	part, err := w.CreatePart(headers)
	if err != nil {
		return err
	}
	// A 57-byte chunk produces the standard maximum of 76 Base64 characters.
	const chunk = 57
	var encoded [76]byte
	for offset := 0; offset < len(data); offset += chunk {
		if offset > 0 {
			if _, err := io.WriteString(part, "\r\n"); err != nil {
				return err
			}
		}
		block := data[offset:min(offset+chunk, len(data))]
		base64.StdEncoding.Encode(encoded[:], block)
		if _, err := part.Write(encoded[:base64.StdEncoding.EncodedLen(len(block))]); err != nil {
			return err
		}
	}
	return nil
}
