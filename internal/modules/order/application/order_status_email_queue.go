package application

import (
	"strings"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/telegramidentity"
)

type receiverEmailResolver interface {
	ResolveReceiverEmailByOrderID(orderID uint) (string, error)
}

// EnqueueStatusEmailTaskIfEligible 根据订单接收邮箱策略决定是否入队状态邮件任务。
// 返回值 skipped 表示任务被策略跳过（例如 Telegram 占位邮箱）。
func EnqueueStatusEmailTaskIfEligible(
	orderStore receiverEmailResolver,
	queueClient ordercontract.Queue,
	settingService *settingsapp.Service,
	defaultEmailConfig config.EmailConfig,
	orderID uint,
	status string,
) (skipped bool, err error) {
	if queueClient == nil || orderID == 0 {
		return true, nil
	}
	if settingService != nil {
		smtpSetting, smtpErr := settingService.GetSMTPSetting(defaultEmailConfig)
		if smtpErr != nil {
			return false, smtpErr
		}
		if !smtpSetting.Enabled {
			return true, nil
		}
		if !smtpSetting.IsOrderStatusEmailEnabled(status) {
			return true, nil
		}
	}
	if orderStore == nil {
		if err := queueClient.EnqueueStatusEmail(orderID, strings.TrimSpace(status)); err != nil {
			return false, err
		}
		return false, nil
	}

	receiverEmail, lookupErr := orderStore.ResolveReceiverEmailByOrderID(orderID)
	if lookupErr == nil {
		receiverEmail = strings.TrimSpace(receiverEmail)
		if receiverEmail == "" {
			return true, nil
		}
		if telegramidentity.IsPlaceholderEmail(receiverEmail) {
			return true, nil
		}
	}

	if err := queueClient.EnqueueStatusEmail(orderID, strings.TrimSpace(status)); err != nil {
		return false, err
	}
	return false, nil
}
