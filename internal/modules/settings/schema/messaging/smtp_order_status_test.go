package settingsmessaging

import (
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestIsOrderStatusEmailEnabled(t *testing.T) {
	setting := DefaultSMTPSetting(config.EmailConfig{Enabled: true})
	setting.OrderNotificationStatuses.Completed = false

	cases := map[string]bool{
		constants.OrderStatusPaid:              true,
		constants.OrderStatusDelivered:         true,
		constants.OrderStatusCompleted:         false,
		constants.OrderStatusRefunded:          true,
		constants.OrderStatusPartiallyRefunded: true,
	}
	for status, want := range cases {
		if got := setting.IsOrderStatusEmailEnabled(status); got != want {
			t.Errorf("status %s: got %v want %v", status, got, want)
		}
	}

	setting.OrderNotificationEnabled = false
	if setting.IsOrderStatusEmailEnabled(constants.OrderStatusDelivered) {
		t.Error("master switch off should disable all statuses")
	}
}

func TestDecodeSMTPSettingLegacyDefaultsStatusesOn(t *testing.T) {
	fallback := DefaultSMTPSetting(config.EmailConfig{Enabled: true})
	decoded := DecodeSMTPSetting(jsonmap.JSON{"order_notification_enabled": true}, fallback)
	if !decoded.OrderNotificationStatuses.Completed || !decoded.OrderNotificationStatuses.Delivered {
		t.Error("legacy settings without statuses must keep all emails on")
	}
}

func TestApplySMTPSettingPatchStatuses(t *testing.T) {
	current := DefaultSMTPSetting(config.EmailConfig{})
	off := false
	next, err := ApplySMTPSettingPatch(current, SMTPSettingPatch{
		OrderNotificationStatuses: &SMTPOrderNotificationStatusesPatch{Completed: &off},
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.OrderNotificationStatuses.Completed || !next.OrderNotificationStatuses.Delivered {
		t.Errorf("unexpected statuses: %+v", next.OrderNotificationStatuses)
	}
}
