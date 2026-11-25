package smtp

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

func TestBrandedOrderEmailPreservesSavedVariablesAndAttachment(t *testing.T) {
	brand := mailbrand.Brand{SiteName: "示例商城", SiteURL: "https://shop.example.test"}
	input := notificationcontract.OrderStatusEmailInput{
		Status: constants.OrderStatusDelivered, OrderNo: "TEST-1001", SiteName: brand.SiteName, SiteURL: brand.SiteURL,
		FulfillmentInfo: "货品 A < B & C",
	}
	setting := settingsmessaging.DefaultOrderEmailTemplateSetting()
	setting.Templates.DeliveredWithContent.ZHCN.Subject = "交付 {{order_no}}"
	setting.Templates.DeliveredWithContent.ZHCN.Body = "订单 {{order_no}}\n{{fulfillment_info}}"
	subject, body := buildOrderStatusContentFromTemplate(input, i18n.LocaleZH, setting)
	attachment := strings.Repeat("测试交付内容 / TEST-1001\n", 200)
	raw, err := buildEmailMessage("from@example.test", "buyer@example.test", emailMessage{
		Subject: subject, Text: body, Brand: brand, Layout: textEmailLayout(subject, body, i18n.LocaleZH, brand),
		AttachmentName: "交付内容.txt", AttachmentContent: attachment,
	})
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("overlong MIME line: %d bytes", len(line))
		}
	}
	mailMessage, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subjectDecoded, err := (&mime.WordDecoder{}).DecodeHeader(mailMessage.Header.Get("Subject"))
	if err != nil || subjectDecoded != "交付 TEST-1001" {
		t.Fatalf("subject=%q err=%v", subjectDecoded, err)
	}
	mixed := multipartReader(t, mailMessage.Body, mailMessage.Header.Get("Content-Type"), "multipart/mixed")
	alternativesPart := nextMailPart(t, mixed)
	alternatives := multipartReader(t, alternativesPart, alternativesPart.Header.Get("Content-Type"), "multipart/alternative")
	plainPart := nextMailPart(t, alternatives)
	if plain := decodeMailPart(t, plainPart); !strings.HasPrefix(string(plain), "订单 TEST-1001\n货品 A < B & C") {
		t.Fatalf("missing rendered plain text: %q", plain)
	}
	html := string(decodeMailPart(t, nextMailPart(t, alternatives)))
	for _, want := range []string{"示例商城", "订单 TEST-1001<br>货品 A &lt; B &amp; C", "bgcolor=\"#17181c\""} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	attachmentPart := nextMailPart(t, mixed)
	disposition, values, err := mime.ParseMediaType(attachmentPart.Header.Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || values["filename"] != "交付内容.txt" {
		t.Fatalf("attachment name failed: %q %v", values["filename"], err)
	}
	if content := decodeMailPart(t, attachmentPart); string(content) != attachment {
		t.Fatal("attachment changed during encoding")
	}

	// Saving different order copy must not remove the HTML wrapper.
	setting.Templates.DeliveredWithContent.ZHCN.Body = "更新后的文案 {{site_name}}"
	subject, body = buildOrderStatusContentFromTemplate(input, i18n.LocaleZH, setting)
	var updated bytes.Buffer
	if err := brandedEmailTemplate.Execute(&updated, textEmailLayout(subject, body, i18n.LocaleZH, brand)); err != nil {
		t.Fatal(err)
	}
	if html := updated.String(); !strings.Contains(html, "示例商城") || !strings.Contains(html, "更新后的文案 示例商城") {
		t.Fatal("saved template content lost the branded layout")
	}
}

func TestVerificationLayoutUsesActualCodeAndTenant(t *testing.T) {
	brand := mailbrand.Brand{SiteName: "Example Store", SiteURL: "https://shop.example.test"}
	var html bytes.Buffer
	view := verificationEmailLayout("Example Store - Password Reset Code", "1234567890",
		constants.VerifyPurposeReset, i18n.LocaleEN, 7, brand)
	if err := brandedEmailTemplate.Execute(&html, view); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Password Reset Code", "1234567890", "Valid for 7 minutes", "shop.example.test", `class="verification-code-long"`, "font-size: 24px"} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func multipartReader(t *testing.T, reader io.Reader, contentType, want string) *multipart.Reader {
	t.Helper()
	mediaType, values, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != want || values["boundary"] == "" {
		t.Fatalf("content type = %q, want %q, err=%v", contentType, want, err)
	}
	return multipart.NewReader(reader, values["boundary"])
}

func nextMailPart(t *testing.T, reader *multipart.Reader) *multipart.Part {
	t.Helper()
	part, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func decodeMailPart(t *testing.T, part *multipart.Part) []byte {
	t.Helper()
	content, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
	if err != nil {
		t.Fatal(err)
	}
	return content
}
