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

		paymentRepo := tx.Payments()
		channelRepo := tx.PaymentChannels()
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
					if _, err := paymentRepo.SupersedePendingByOrderID(lockedOrder.ID, existing.ID, time.Now()); err != nil {
						return ErrPaymentCreateFailed
					}
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
			// 与在线支付回调一致：分销利润与支付在同一事务入账
			if s.resellerAccounting != nil {
				if err := s.resellerAccounting.PostOrderProfit(tx.ResellerAccounting(), &lockedOrder, payment); err != nil {
					return err
				}
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
		// 先落库网关订单号，早于创建响应到达的回调才能定位到这条支付。
		payment.GatewayOrderNo = resolveGatewayOrderNo(channel, payment)

		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
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

	// takenOver 非空表示网关请求期间订单已被并发支付接管，记录接管后的订单快照。
	var takenOver *orderdomain.Order
	if err := s.applyProviderPayment(input, order, channel, payment); err != nil {
		rollbackErr := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
			lockedOrder, current, newerID, lockErr := lockPaymentAttempt(tx, order.ID, payment.ID)
			if lockErr != nil {
				return lockErr
			}
			// 已被钱包支付/新尝试替代或已回调：不能改写它，更不能退还别人的钱包占款。
			if current.Status != constants.PaymentStatusInitiated {
				takenOver = lockedOrder
				return nil
			}
			current.Status = constants.PaymentStatusFailed
			current.UpdatedAt = time.Now()
			if updateErr := tx.Payments().Update(current); updateErr != nil {
				return updateErr
			}
			if s.walletSvc == nil || newerID != 0 || lockedOrder.Status != constants.OrderStatusPendingPayment {
				return nil
			}
			_, refundErr := orderapp.ReleaseWalletBalance(s.walletSvc, tx, lockedOrder, constants.WalletTxnTypeOrderRefund, "在线支付创建失败，退回余额")
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
		if rollbackErr == nil && takenOver != nil {
			return takenOverResult(takenOver)
		}
		return nil, err
	}
	if err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		lockedOrder, current, newerID, err := lockPaymentAttempt(tx, order.ID, payment.ID)
		if err != nil {
			return err
		}
		now := time.Now()
		// 网关迟到成功：订单已不再归这次尝试所有时，作废自己的链接而不是作废更新的支付。
		if lockedOrder.Status != constants.OrderStatusPendingPayment || newerID != 0 || current.Status != constants.PaymentStatusPending {
			takenOver = lockedOrder
			if current.Status != constants.PaymentStatusPending {
				return nil
			}
			current.Status = constants.PaymentStatusExpired
			current.ExpiredAt = &now
			current.UpdatedAt = now
			if newerID != 0 {
				current.SupersededAt = &now
				current.SupersededByPaymentID = &newerID
			}
			return tx.Payments().Update(current)
		}
		_, err = tx.Payments().SupersedePendingByOrderID(order.ID, payment.ID, now)
		return err
	}); err != nil {
		log.Errorw("payment_create_supersede_previous_failed",
			"payment_id", payment.ID,
			"order_id", order.ID,
			"error", err,
		)
		return nil, ErrPaymentCreateFailed
	}
	if takenOver != nil {
		log.Warnw("payment_create_stale_provider_success", "payment_id", payment.ID, "order_id", order.ID)
		return takenOverResult(takenOver)
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

// lockPaymentAttempt 网关返回后按"先订单后支付"重新加锁读取（与创建、钱包支付、回调同序），
// 并返回同订单上比该尝试更新且仍有效的支付 ID；非零说明钱包占款与订单已归更新的尝试所有。
func lockPaymentAttempt(tx paymentcontract.Transaction, orderID, paymentID uint) (*orderdomain.Order, *paymentdomain.Payment, uint, error) {
	order, err := tx.Orders().GetByIDForUpdate(orderID)
	if err != nil {
		return nil, nil, 0, orderapp.ErrOrderFetchFailed
	}
	if order == nil {
		return nil, nil, 0, orderapp.ErrOrderNotFound
	}
	current, err := tx.Payments().GetByIDForUpdate(paymentID)
	if err != nil {
		return nil, nil, 0, ErrPaymentUpdateFailed
	}
	if current == nil {
		return nil, nil, 0, ErrPaymentNotFound
	}
	payments, err := tx.Payments().ListByOrderID(orderID)
	if err != nil {
		return nil, nil, 0, ErrPaymentUpdateFailed
	}
	return order, current, newerActivePaymentID(payments, paymentID), nil
}

// newerActivePaymentID 返回同订单上比 paymentID 更新且仍有效的支付；非零时钱包占款已不归 paymentID。
func newerActivePaymentID(payments []paymentdomain.Payment, paymentID uint) uint {
	for _, p := range payments {
		if p.ID > paymentID && (p.Status == constants.PaymentStatusInitiated || p.Status == constants.PaymentStatusPending || p.Status == constants.PaymentStatusSuccess) {
			return p.ID
		}
	}
	return 0
}

// takenOverResult 旧尝试被并发支付接管：订单已付清就如实返回已支付（与钱包全额支付同形），
// 否则返回各入口（网页、Bot、钱包）都已映射的订单状态错误，提示用户刷新。
func takenOverResult(order *orderdomain.Order) (*CreatePaymentResult, error) {
	if order.PaidAt != nil {
		return &CreatePaymentResult{OrderPaid: true, WalletPaidAmount: order.WalletPaidAmount, OnlinePayAmount: order.OnlinePaidAmount}, nil
	}
	return nil, orderapp.ErrOrderStatusInvalid
}
