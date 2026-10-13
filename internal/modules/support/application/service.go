package application

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	Store  contract.Store
	orders contract.Orders
	agents contract.Agents
}

func New(store contract.Store, orders contract.Orders, agents contract.Agents) *Service {
	return &Service{store, orders, agents}
}
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func (s *Service) Get(ctx context.Context, a contract.Actor, id uint) (*domain.Conversation, error) {
	c, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.AdminID > 0 {
		if !a.Supervisor && c.AssigneeID != 0 && c.AssigneeID != a.AdminID {
			return nil, contract.ErrNotFound
		}
		return c, nil
	}
	if c.TenantID != a.TenantID {
		return nil, contract.ErrNotFound
	}
	if a.UserID > 0 && c.UserID == a.UserID {
		return c, nil
	}
	if a.UserID == 0 && c.UserID == 0 && len(a.GuestToken) == 64 && c.GuestHash == HashToken(a.GuestToken) && c.GuestExpiresAt.After(time.Now()) {
		return c, nil
	}
	return nil, contract.ErrNotFound
}

type CreateInput struct {
	Subject   string `json:"subject"`
	Category  string `json:"category"`
	ProductID uint   `json:"product_id"`
	OrderNo   string `json:"order_no"`
	Content   string `json:"content"`
	ClientID  string `json:"client_id"`
}
type MessageInput struct {
	Content       string `json:"content"`
	ClientID      string `json:"client_id"`
	Internal      bool   `json:"internal"`
	AttachmentIDs []uint `json:"attachment_ids"`
}

func validMessage(content, clientID string, attachments int) bool {
	return (strings.TrimSpace(content) != "" || attachments > 0) && utf8.RuneCountInString(content) <= 4000 && len(content) <= 16000 && len(clientID) > 0 && len(clientID) <= 64 && attachments <= 4
}
func (s *Service) Create(ctx context.Context, a contract.Actor, in CreateInput, email, password string) (*domain.Conversation, string, error) {
	if a.AdminID > 0 || !validMessage(in.Content, in.ClientID, 0) || utf8.RuneCountInString(in.Subject) > 160 {
		return nil, "", contract.ErrInvalid
	}
	switch in.Category {
	case "general", "presale", "payment", "delivery", "aftersale":
	default:
		in.Category = "general"
	}
	c := &domain.Conversation{TenantID: a.TenantID, UserID: a.UserID, Subject: strings.TrimSpace(in.Subject), Category: in.Category, ProductID: in.ProductID, Status: "open"}
	if c.Subject == "" {
		c.Subject = in.Category
	}
	token := ""
	if a.UserID == 0 {
		if len(a.GuestToken) == 64 {
			token = a.GuestToken
		} else {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				return nil, "", err
			}
			token = hex.EncodeToString(b)
		}
		c.GuestHash = HashToken(token)
		c.GuestExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}
	c.CreationKey = HashToken(fmt.Sprintf("%d:%d:%s:%s", a.TenantID, a.UserID, c.GuestHash, in.ClientID))
	if in.OrderNo != "" {
		summary, err := s.orders.Verify(ctx, a, in.OrderNo, email, password)
		if err != nil {
			return nil, "", contract.ErrForbidden
		}
		data, _ := json.Marshal(summary)
		c.OrderSummary = string(data)
	}
	m := &domain.Message{ClientID: in.ClientID, Content: strings.TrimSpace(in.Content), SenderType: "guest", SenderID: a.UserID}
	if a.UserID > 0 {
		m.SenderType = "user"
	}
	if err := s.Store.Create(ctx, c, m); err != nil {
		return nil, "", err
	}
	return c, token, nil
}
func (s *Service) List(ctx context.Context, a contract.Actor, status string, page, limit int) ([]domain.Conversation, int64, error) {
	if a.AdminID == 0 && a.UserID == 0 {
		if len(a.GuestToken) != 64 {
			return []domain.Conversation{}, 0, nil
		}
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	return s.Store.List(ctx, a, status, page, limit)
}
func (s *Service) Messages(ctx context.Context, a contract.Actor, id, after uint, limit int) ([]domain.Message, error) {
	if _, err := s.Get(ctx, a, id); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	return s.Store.Messages(ctx, id, after, limit, a.AdminID > 0)
}
func (s *Service) Append(ctx context.Context, a contract.Actor, id uint, in MessageInput) (*domain.Message, error) {
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return nil, err
	}
	if !validMessage(in.Content, in.ClientID, len(in.AttachmentIDs)) || (in.Internal && a.AdminID == 0) {
		return nil, contract.ErrInvalid
	}
	if a.AdminID > 0 && c.AssigneeID != a.AdminID && !a.Supervisor {
		return nil, contract.ErrForbidden
	}
	m := &domain.Message{ConversationID: id, ClientID: in.ClientID, Content: strings.TrimSpace(in.Content), Internal: in.Internal, SenderType: "guest", SenderID: a.UserID}
	if a.AdminID > 0 {
		m.SenderType = "admin"
		m.SenderID = a.AdminID
	} else if a.UserID > 0 {
		m.SenderType = "user"
	}
	return s.Store.Append(ctx, c, m, in.AttachmentIDs)
}
func (s *Service) LinkOrder(ctx context.Context, a contract.Actor, id uint, no, email, password string) error {
	if a.AdminID > 0 {
		return contract.ErrForbidden
	}
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return err
	}
	v, err := s.orders.Verify(ctx, a, no, email, password)
	if err != nil {
		return contract.ErrForbidden
	}
	data, _ := json.Marshal(v)
	c.OrderSummary = string(data)
	return s.Store.Update(ctx, c, a, "order", 0)
}
func (s *Service) Claim(ctx context.Context, a contract.Actor, id uint) error {
	if a.AdminID == 0 {
		return contract.ErrForbidden
	}
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return err
	}
	c.AssigneeID = a.AdminID
	c.Status = "active"
	return s.Store.Update(ctx, c, a, "claim", a.AdminID)
}
func (s *Service) Assign(ctx context.Context, a contract.Actor, id, target uint) error {
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return err
	}
	if a.AdminID == 0 || (!a.Supervisor && c.AssigneeID != a.AdminID) || target == 0 || !s.agents.Eligible(ctx, target) {
		return contract.ErrForbidden
	}
	c.AssigneeID = target
	return s.Store.Update(ctx, c, a, "assign", target)
}
func (s *Service) Status(ctx context.Context, a contract.Actor, id uint, status string) error {
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return err
	}
	if a.AdminID == 0 || (!a.Supervisor && c.AssigneeID != a.AdminID) {
		return contract.ErrForbidden
	}
	switch status {
	case "open", "active", "waiting", "resolved":
	default:
		return contract.ErrInvalid
	}
	c.Status = status
	return s.Store.Update(ctx, c, a, "status", 0)
}
func (s *Service) Read(ctx context.Context, a contract.Actor, id, msg uint) error {
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return err
	}
	if a.AdminID > 0 && c.AssigneeID != a.AdminID && !a.Supervisor {
		return contract.ErrForbidden
	}
	return s.Store.Read(ctx, id, msg, a.AdminID > 0)
}
func (s *Service) Agents(ctx context.Context) ([]contract.Agent, error) { return s.agents.List(ctx) }
func (s *Service) Upload(ctx context.Context, a contract.Actor, id uint, mime string, data []byte) (*domain.Attachment, error) {
	c, err := s.Get(ctx, a, id)
	if err != nil {
		return nil, err
	}
	if a.AdminID > 0 && !a.Supervisor && c.AssigneeID != a.AdminID {
		return nil, contract.ErrForbidden
	}
	if len(data) == 0 || len(data) > 5*1024*1024 {
		return nil, contract.ErrInvalid
	}
	cfg, _, decodeErr := image.DecodeConfig(bytes.NewReader(data))
	if decodeErr != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 12000 || cfg.Height > 12000 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return nil, contract.ErrInvalid
	}
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, contract.ErrInvalid
	}
	v := &domain.Attachment{ConversationID: id, SenderType: "guest", SenderID: a.UserID, MIME: mime, Size: len(data), Data: data}
	if a.UserID > 0 {
		v.SenderType = "user"
	}
	if a.AdminID > 0 {
		v.SenderType = "admin"
		v.SenderID = a.AdminID
	}
	err = s.Store.PutAttachment(ctx, v)
	return v, err
}
func (s *Service) Attachment(ctx context.Context, a contract.Actor, id, attachment uint) (*domain.Attachment, error) {
	if _, err := s.Get(ctx, a, id); err != nil {
		return nil, err
	}
	v, err := s.Store.GetAttachment(ctx, id, attachment)
	if err != nil {
		return nil, err
	}
	if a.AdminID == 0 {
		if v.MessageID == 0 {
			if (a.UserID > 0 && (v.SenderType != "user" || v.SenderID != a.UserID)) || (a.UserID == 0 && v.SenderType != "guest") {
				return nil, contract.ErrNotFound
			}
		} else {
			ms, err := s.Store.Messages(ctx, id, v.MessageID-1, 1, false)
			if err != nil || len(ms) != 1 || ms[0].ID != v.MessageID {
				return nil, contract.ErrNotFound
			}
		}
	}
	return v, nil
}
