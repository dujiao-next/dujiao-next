package automation

import (
	"context"
	"errors"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"gorm.io/gorm"
	"strings"
	"time"
)

// Operate requires OS/database operator access; no HTTP or Telegram binding.
// It cannot mark a payment successful, deliver goods, delete evidence or refund.
func Operate(ctx context.Context, db *gorm.DB, action, operator, reason string, revision uint64, paymentID uint) error {
	if strings.TrimSpace(operator) == "" || len(operator) > 120 || strings.TrimSpace(reason) == "" || len(reason) > 160 {
		return errors.New("operator and bounded reason required")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := pd.OperatorAudit{Operator: operator, Action: action, PreviousRevision: revision, PaymentID: paymentID, Reason: reason, CreatedAt: time.Now()}
		switch action {
		case "freeze", "resume":
			if err := SetFrozen(ctx, tx, action == "freeze", revision, reason); err != nil {
				return err
			}
			row.Revision = revision + 1
		case "note":
			var payment pd.Payment
			if paymentID == 0 || tx.Unscoped().First(&payment, paymentID).Error != nil || !payment.RequiresChainVerification {
				return errors.New("fixed payment required")
			}
		default:
			return errors.New("unsupported operator action")
		}
		return tx.Create(&row).Error
	})
}
