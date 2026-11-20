package container

import (
	"errors"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	alipayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/alipay"
	bepusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/bepusdt"
	dujiaopayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/dujiaopay"
	epayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epay"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	nanoadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/nano"
	okpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/okpay"
	paypaladapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/paypal"
	stripeadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/stripe"
	tokenpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/tokenpay"
	wechatpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/wechatpay"
	paymentprovider "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/queue"
)

// NewContainer 初始化应用依赖容器。
func NewContainer(cfg *config.Config) (*Container, error) {
	if cfg == nil {
		return nil, errors.New("container config is nil")
	}
	if err := cache.InitRedis(&cfg.Redis); err != nil {
		logger.Warnw("provider_init_redis_failed", "error", err)
	}

	var queueClient *queue.Client
	if cfg.Queue.Enabled {
		qc, err := queue.NewClient(&cfg.Queue)
		if err != nil {
			logger.Errorw("provider_init_queue_client_failed", "error", err)
		} else {
			queueClient = qc
		}
	}

	c := &Container{
		Config:                  cfg,
		QueueClient:             queueClient,
		PaymentProviderRegistry: newPaymentProviderRegistry(),
	}
	if err := c.initRepositories(); err != nil {
		return nil, err
	}
	c.initServices()
	return c, nil
}

// newPaymentProviderRegistry 注册应用支持的全部支付适配器。
// PaymentService 构造时依赖完整注册表，因此该步骤必须先于 Service 装配。
func newPaymentProviderRegistry() *paymentprovider.Registry {
	registry := paymentprovider.NewRegistry()
	registry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeStripe, stripeadapter.NewStripeAdapter())
	registry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypePaypal, paypaladapter.NewPaypalAdapter())
	registry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeWechat, wechatpayadapter.NewWechatpayAdapter())
	registry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeAlipay, alipayadapter.NewAlipayAdapter())
	registry.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeNano, nanoadapter.NewNanoAdapter())
	registry.Register(constants.PaymentProviderEpay, "", epayadapter.NewEpayAdapter())
	registry.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	registry.Register(constants.PaymentProviderBepusdt, "", bepusdtadapter.NewBepusdtAdapter())
	registry.Register(constants.PaymentProviderDujiaoPay, "", dujiaopayadapter.NewDujiaoPayAdapter())
	registry.Register(constants.PaymentProviderTokenpay, "", tokenpayadapter.NewTokenpayAdapter())
	registry.Register(constants.PaymentProviderOkpay, "", okpayadapter.NewOkpayAdapter())
	return registry
}
