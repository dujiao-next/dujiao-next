package application

import (
	"context"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CreatePaymentInput 创建支付请求
type CreatePaymentInput struct {
	OrderID          uint
	ChannelID        uint
	UseBalance       bool
	ClientIP         string
	Context          context.Context
	ReturnBizType    string
	ReturnBusinessNo string
	ReturnGuest      bool
	// RequestScheme 当前请求的 scheme（http/https），用于分销站动态生成 return_url；空值默认 https。
	RequestScheme string
}

// CreatePaymentResult 创建支付结果
type CreatePaymentResult struct {
	Payment          *paymentdomain.Payment
	Channel          *paymentdomain.PaymentChannel
	OrderPaid        bool
	WalletPaidAmount money.Amount
	OnlinePayAmount  money.Amount
}

func hasProviderResult(payment *paymentdomain.Payment) bool {
	if payment == nil {
		return false
	}
	if payment.RequiresChainVerification {
		return len(payment.PaymentIntent) != 0 && payment.ProviderRef != ""
	}
	return strings.TrimSpace(payment.PayURL) != "" || strings.TrimSpace(payment.QRCode) != ""
}

// CreatePayment 创建支付单
func (s *PaymentService) CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error) {
	if input.OrderID == 0 {
		return nil, ErrPaymentInvalid
	}

	log := paymentLogger(
		"order_id", input.OrderID,
		"channel_id", input.ChannelID,
	)

	var payment *paymentdomain.Payment
	var order *orderdomain.Order
	var channel *paymentdomain.PaymentChannel
	feeRate := decimal.Zero
	reusedPending := false
	resumeIntent := false
	orderPaidByWallet := false
	now := time.Now()
	feeConfig := settingsapp.DefaultPaymentFeeConfig()
	if s.settingService != nil {
		feeConfig = s.settingService.GetPaymentFeeConfig()
	}

	// 在事务外查询设置，避免 SQLite 单连接池下自锁
	walletOnly := s.settingService != nil && s.settingService.GetWalletOnlyPayment()
	if walletOnly {
		input.UseBalance = true
		if input.ChannelID != 0 {
			return nil, walletcontract.ErrOnlyPaymentRequired
		}
	}

	err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		var selected *paymentdomain.PaymentChannel
		if input.ChannelID != 0 {
			var err error
			selected, err = tx.PaymentChannels().GetByID(input.ChannelID)
			if err != nil {
				return err
			}
		}
		revision, err := s.lockPaymentCreation(tx, selected, input.OrderID)
		if err != nil {
			return err
		}
		preloaded, err := tx.Orders().GetByIDForUpdateWithChildren(input.OrderID)
		if err != nil {
			return orderapp.ErrOrderFetchFailed
		}
		if preloaded == nil {
			return orderapp.ErrOrderNotFound
		}
		lockedOrder := *preloaded
		if lockedOrder.ParentID != nil {
			return ErrPaymentInvalid
		}
		if lockedOrder.Status != constants.OrderStatusPendingPayment {
			return orderapp.ErrOrderStatusInvalid
		}
		if lockedOrder.ExpiresAt != nil && !lockedOrder.ExpiresAt.After(time.Now()) {
			return orderapp.ErrOrderStatusInvalid
		}

		if revision != 0 {
			if err := validateFixedPaymentItems(&lockedOrder); err != nil {
				return err
			}
		}
		paymentRepo := tx.Payments()
		channelRepo := tx.PaymentChannels()
		// Preserve the wallet split and identity while a fixed intent is active,
		// including the gap between SQL commit and the first gateway response.
		prior, err := paymentRepo.ListByOrderID(lockedOrder.ID)
		if err != nil {
			return ErrPaymentCreateFailed
		}
		for idx := range prior {
			p := &prior[idx]
			if !p.RequiresChainVerification || p.SupersededAt != nil || (p.Status != constants.PaymentStatusInitiated && p.Status != constants.PaymentStatusPending) || (p.ExpiredAt != nil && !p.ExpiredAt.After(time.Now())) {
				continue
			}
			if p.ChannelID != input.ChannelID {
				return ErrPaymentStatusInvalid
			}
			// Another request may have created this intent while we waited for
			// the order lock. Retry from the start instead of taking locks backwards.
			if revision == 0 {
				return paymentdomain.ErrAutomationFrozen
			}
			resolved, err := channelRepo.GetByID(p.ChannelID)
			if err != nil || resolved == nil {
				return ErrPaymentChannelNotFound
			}
			if err := validateOrderChannelEligibility(*resolved, &lockedOrder); err != nil {
				return err
			}
			if len(p.PaymentIntent) == 0 {
				return ErrPaymentInvalid
			}
			p.AutomationRevision = revision
			payment, order, channel = p, &lockedOrder, resolved
			reusedPending = hasProviderResult(p)
			resumeIntent = !reusedPending
			return nil
		}

		if input.ChannelID != 0 {
			if channel == nil {
				// 事务内必须使用 tx 绑定仓储，避免在单连接池下发生自锁等待。
				resolvedChannel, err := channelRepo.GetByID(input.ChannelID)
				if err != nil {
					return err
				}
				if resolvedChannel == nil {
					return ErrPaymentChannelNotFound
				}
				if !resolvedChannel.IsActive {
					return ErrPaymentChannelInactive
				}
				resolvedFeeRate := resolvedChannel.FeeRate.Decimal.Round(2)
				if resolvedFeeRate.LessThan(decimal.Zero) || resolvedFeeRate.GreaterThan(decimal.NewFromInt(100)) {
					return ErrPaymentChannelConfigInvalid
				}
				channel = resolvedChannel
				feeRate = resolvedFeeRate
			}
			if err := validateOrderChannelEligibility(*channel, &lockedOrder); err != nil {
				return err
			}

			// 校验商品是否允许该支付渠道（传入 tx 避免 SQLite 自锁）
			allItems := lockedOrder.Items
			for _, child := range lockedOrder.Children {
				allItems = append(allItems, child.Items...)
			}
			if err := s.validateProductPaymentChannel(allItems, channel.ID, tx.Products()); err != nil {
				return err
			}

			existing, err := paymentRepo.GetLatestPendingByOrderChannel(lockedOrder.ID, channel.ID, time.Now())
			if err != nil {
				return ErrPaymentCreateFailed
			}
			if existing != nil && hasProviderResult(existing) {
				legacyFeePayment := existing.FeeAmount.Decimal.IsPositive() &&
					(existing.FeePolicy == "" || existing.FeePolicy == constants.PaymentFeePolicyLegacyCustomerSurcharge)
				if !legacyFeePayment || feeConfig.ReuseLegacyOrderFeePayment {
					reusedPending = true
					payment = existing
					order = &lockedOrder
					return nil
				}
			}
		}

		if s.walletSvc != nil {
			if input.UseBalance {
				if _, err := orderapp.ApplyWalletBalance(s.walletSvc, tx, &lockedOrder, true); err != nil {
					return err
				}
			} else if lockedOrder.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
				if _, err := orderapp.ReleaseWalletBalance(s.walletSvc, tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "用户改为在线支付，退回余额"); err != nil {
					return err
				}
			}
		}

		onlineAmount := normalizeOrderAmount(lockedOrder.TotalAmount.Decimal.Sub(lockedOrder.WalletPaidAmount.Decimal))
		if onlineAmount.LessThanOrEqual(decimal.Zero) {
			walletPaidAmount := normalizeOrderAmount(lockedOrder.WalletPaidAmount.Decimal)
			paidAt := time.Now()
			payment = &paymentdomain.Payment{
				OrderID:         lockedOrder.ID,
				ChannelID:       0,
				ProviderType:    constants.PaymentProviderWallet,
				ChannelType:     constants.PaymentChannelTypeBalance,
				InteractionMode: constants.PaymentInteractionBalance,
				Amount:          money.FromDecimal(walletPaidAmount),
				FeeRate:         money.FromDecimal(decimal.Zero),
				FixedFee:        money.FromDecimal(decimal.Zero),
				FeeAmount:       money.FromDecimal(decimal.Zero),
				FeePolicy:       constants.PaymentFeePolicyNone,
				Currency:        lockedOrder.Currency,
				Status:          constants.PaymentStatusSuccess,
				CreatedAt:       paidAt,
				UpdatedAt:       paidAt,
				PaidAt:          &paidAt,
			}
			if err := paymentRepo.Create(payment); err != nil {
				return ErrPaymentCreateFailed
			}
			// 余额已覆盖全额，订单遗留的在线链接必须同时作废，避免用户再付一次。
			if _, err := paymentRepo.SupersedePendingByOrderID(lockedOrder.ID, payment.ID, paidAt); err != nil {
				return ErrPaymentCreateFailed
			}
			if err := s.markOrderPaid(tx, &lockedOrder, paidAt); err != nil {
				return err
			}
			orderPaidByWallet = true
			order = &lockedOrder
			return nil
		}
		if channel == nil {
			if walletOnly {
				return walletcontract.ErrOnlyPaymentRequired
			}
			return ErrPaymentInvalid
		}
		if err := validatePaymentCurrencyForChannel(lockedOrder.Currency, channel); err != nil {
			return err
		}
		if err := validatePaymentAmountForChannel(onlineAmount, channel); err != nil {
			return err
		}

		fixedFee := decimal.Zero
		if channel.FixedFee.Decimal.GreaterThan(decimal.Zero) {
			fixedFee = channel.FixedFee.Decimal.Round(2)
		}

		paymentAmount, feeAmount, feePolicy := calculatePaymentAmounts(onlineAmount, feeRate, fixedFee, feeConfig.CustomerFeeEnabled)
		payment = &paymentdomain.Payment{
			OrderID:         lockedOrder.ID,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          money.FromDecimal(paymentAmount),
			FeeRate:         money.FromDecimal(feeRate),
			FixedFee:        money.FromDecimal(fixedFee),
			FeeAmount:       money.FromDecimal(feeAmount),
			FeePolicy:       feePolicy,
			Currency:        lockedOrder.Currency,
			Status:          constants.PaymentStatusInitiated,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if shouldUseCNYPaymentCurrency(channel) {
			payment.Currency = "CNY"
		}

		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}
		// Authorize in the SAME transaction as payment creation. A process
		// crash after this commit must leave a complete resumable request.
		if s.paymentProviderRegistry != nil {
			if provider, ok := s.paymentProviderRegistry.Lookup(channel.ProviderType, channel.ChannelType); ok {
				if preparer, ok := provider.(paymentcontract.GatewayIntentPreparer); ok {
					payment.GatewayOrderNo = resolveGatewayOrderNo(channel, payment)
					createInput := paymentcontract.GatewayCreateInput{AutomationRevision: revision, PaymentID: payment.ID, OrderID: lockedOrder.ID, OrderNo: payment.GatewayOrderNo,
						Subject: buildOrderSubject(&lockedOrder), Amount: payment.Amount, Currency: payment.Currency, ExpiresAt: lockedOrder.ExpiresAt,
						ReturnURL: resolveTenantReturnURL(input.Context, input.RequestScheme, channel), ReturnURLQuery: buildPaymentReturnQuery(input, &lockedOrder, channel.ProviderType+"_return", "")}
					if err := s.prepareGatewayPaymentIntentTx(tx, preparer, channel, payment, &createInput); err != nil {
						return err
					}
				}
			}
		}
		if err := tx.Orders().UpdateFields(lockedOrder.ID, map[string]interface{}{
			"online_paid_amount": money.FromDecimal(onlineAmount),
			"updated_at":         time.Now(),
		}); err != nil {
			return orderapp.ErrOrderUpdateFailed
		}
		lockedOrder.OnlinePaidAmount = money.FromDecimal(onlineAmount)
		lockedOrder.UpdatedAt = time.Now()
		order = &lockedOrder
		return nil
	})
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, orderapp.ErrOrderFetchFailed
	}

	if reusedPending {
		if _, err := s.paymentRepo.SupersedePendingByOrderID(order.ID, payment.ID, time.Now()); err != nil {
			return nil, ErrPaymentCreateFailed
		}
		log.Infow("payment_create_reuse_pending",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
		)
		return &CreatePaymentResult{
			Payment:          payment,
			Channel:          channel,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  order.OnlinePaidAmount,
		}, nil
	}

	if orderPaidByWallet {
		log.Infow("payment_create_wallet_success",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
			"interaction_mode", payment.InteractionMode,
			"currency", payment.Currency,
			"amount", payment.Amount.String(),
			"wallet_paid_amount", order.WalletPaidAmount.String(),
			"online_pay_amount", order.OnlinePaidAmount.String(),
		)
		s.enqueueOrderPaidAsync(order, payment, log)
		return &CreatePaymentResult{
			Payment:          nil,
			Channel:          nil,
			OrderPaid:        true,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  money.FromDecimal(decimal.Zero),
		}, nil
	}

	if payment == nil {
		return nil, ErrPaymentCreateFailed
	}

	if resumeIntent {
		repo, ok := s.paymentRepo.(paymentcontract.IntentRecoveryStore)
		if !ok {
			return nil, ErrPaymentUpdateFailed
		}
		now := time.Now()
		claimed, err := repo.ClaimIntentRecovery(payment.ID, now, now.Add(time.Minute))
		if err != nil {
			return nil, ErrPaymentUpdateFailed
		}
		if !claimed {
			return nil, ErrPaymentGatewayRequestFailed
		}
	}
	if err := s.applyProviderPayment(input, order, channel, payment); err != nil {
		// HTTP errors and invalid responses cannot prove that the remote order
		// was never created. Keep SQL authorization and reserved wallet funds.
		if payment.RequiresChainVerification {
			return nil, err
		}
		rollbackErr := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
			paymentRepo := tx.Payments()
			payment.Status = constants.PaymentStatusFailed
			payment.UpdatedAt = time.Now()
			if updateErr := paymentRepo.Update(payment); updateErr != nil {
				return updateErr
			}
			if s.walletSvc == nil {
				return nil
			}
			preloaded, findErr := tx.Orders().GetByIDForUpdate(order.ID)
			if findErr != nil {
				return findErr
			}
			if preloaded == nil {
				return orderapp.ErrOrderNotFound
			}
			lockedOrder := *preloaded
			_, refundErr := orderapp.ReleaseWalletBalance(s.walletSvc, tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "在线支付创建失败，退回余额")
			return refundErr
		})
		if rollbackErr != nil {
			log.Errorw("payment_create_provider_failed_with_rollback_error",
				"payment_id", payment.ID,
				"order_id", order.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"provider_error", err,
				"rollback_error", rollbackErr,
			)
		} else {
			log.Errorw("payment_create_provider_failed",
				"payment_id", payment.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"error", err,
			)
		}
		return nil, err
	}
	if _, err := s.paymentRepo.SupersedePendingByOrderID(order.ID, payment.ID, time.Now()); err != nil {
		log.Errorw("payment_create_supersede_previous_failed",
			"payment_id", payment.ID,
			"order_id", order.ID,
			"error", err,
		)
		return nil, ErrPaymentCreateFailed
	}

	log.Infow("payment_create_success",
		"payment_id", payment.ID,
		"provider_type", payment.ProviderType,
		"channel_type", payment.ChannelType,
		"interaction_mode", payment.InteractionMode,
		"currency", payment.Currency,
		"amount", payment.Amount.String(),
		"wallet_paid_amount", order.WalletPaidAmount.String(),
		"online_pay_amount", order.OnlinePaidAmount.String(),
	)

	return &CreatePaymentResult{
		Payment:          payment,
		Channel:          channel,
		WalletPaidAmount: order.WalletPaidAmount,
		OnlinePayAmount:  order.OnlinePaidAmount,
	}, nil
}
