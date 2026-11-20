package nanoadapter

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"path/filepath"
	"strings"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/nano"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CurrencyXNO 是 Nano 的币种代码。
const CurrencyXNO = "XNO"

// xnoDecimals 与 payments.amount 的 decimal(20,2) 一致：XNO 金额按 0.01 计价，
// 0.01 XNO 以下的 raw 位留给每笔支付的标记（见 nano.ExpectedRaw）。
const xnoDecimals = 2

// Config 是 official:nano 渠道的 config_json。
//
//	account       收款地址（nano_...，校验和必须正确）
//	node_url      运营者自己选择的 Nano 节点 RPC 地址，只读，不发送任何密钥
//	claim_dir     已入账区块的记录目录（每个区块哈希一个文件），重启后仍然有效
//	exchange_rate 1 单位订单币种 = 多少 XNO；订单币种本身是 XNO 时可留空
type Config struct {
	Account      string `json:"account"`
	NodeURL      string `json:"node_url"`
	ClaimDir     string `json:"claim_dir"`
	ExchangeRate string `json:"exchange_rate"`
}

// Normalize 实现 gatewaycommon.ConfigNormalizer。
func (c *Config) Normalize() {
	c.Account = strings.TrimSpace(c.Account)
	c.NodeURL = strings.TrimSpace(c.NodeURL)
	c.ClaimDir = strings.TrimSpace(c.ClaimDir)
	c.ExchangeRate = strings.TrimSpace(c.ExchangeRate)
}

// nanoAdapter 是 official:nano 的 paymentcontract.GatewayProvider + paymentcontract.GatewayCapturer 实现。
// Nano 没有网关回调：CreatePayment 生成带精确 raw 金额的 nano: 收款链接，
// QueryPayment（走现有的主动查询 capture 流程）向节点查找这笔金额的 send 区块并校验。
type nanoAdapter struct{}

// NewNanoAdapter 实例化 nano adapter。
func NewNanoAdapter() paymentcontract.GatewayProvider { return &nanoAdapter{} }

var (
	_ paymentcontract.GatewayProvider = (*nanoAdapter)(nil)
	_ paymentcontract.GatewayCapturer = (*nanoAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *nanoAdapter) Type() string {
	return constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeNano
}

func configInvalid(format string, args ...interface{}) error {
	return fmt.Errorf("%w: nano "+format, append([]interface{}{paymentcontract.ErrGatewayConfigInvalid}, args...)...)
}

func (a *nanoAdapter) parseConfig(raw jsonmap.JSON) (*Config, error) {
	cfg, err := gatewaycommon.ParseConfig[Config](raw, paymentcontract.ErrGatewayConfigInvalid)
	if err != nil {
		return nil, err
	}
	account, err := nano.ParseAddress(cfg.Account)
	if err != nil {
		return nil, configInvalid("account: %v", err)
	}
	cfg.Account = account
	u, err := url.Parse(cfg.NodeURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, configInvalid("node_url must be an http(s) URL")
	}
	if cfg.ClaimDir == "" || !filepath.IsAbs(cfg.ClaimDir) {
		return nil, configInvalid("claim_dir must be an absolute directory")
	}
	if cfg.ExchangeRate != "" {
		rate, err := decimal.NewFromString(cfg.ExchangeRate)
		if err != nil || !rate.IsPositive() {
			return nil, configInvalid("exchange_rate %q", cfg.ExchangeRate)
		}
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。official provider 的第二个参数是 interaction_mode。
func (a *nanoAdapter) ValidateConfig(raw jsonmap.JSON, interactionMode string) error {
	switch strings.ToLower(strings.TrimSpace(interactionMode)) {
	case "", constants.PaymentInteractionQR, constants.PaymentInteractionRedirect:
	default:
		return configInvalid("interaction_mode %s", interactionMode)
	}
	_, err := a.parseConfig(raw)
	return err
}

// xnoAmount 把订单金额换算为 XNO（向上取到 0.01，不让商家少收）。
func xnoAmount(cfg *Config, amount decimal.Decimal, currency string) (decimal.Decimal, bool, error) {
	if strings.EqualFold(strings.TrimSpace(currency), CurrencyXNO) {
		return amount, false, nil
	}
	if cfg.ExchangeRate == "" {
		return decimal.Zero, false, configInvalid("exchange_rate is required for %s orders", currency)
	}
	rate, _ := decimal.NewFromString(cfg.ExchangeRate)
	return amount.Mul(rate).RoundCeil(xnoDecimals), true, nil
}

// CreatePayment 生成这笔支付专属的 nano: 收款链接。金额 = XNO 价格 × 10^28 raw + 本笔支付的标记，
// 标记由 payment_id 与订单号派生，两笔支付不会是同一个 raw 金额；ProviderRef 就是这个 raw 金额。
func (a *nanoAdapter) CreatePayment(_ context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	orderNo := strings.TrimSpace(input.OrderNo)
	if orderNo == "" || input.PaymentID == 0 {
		return nil, configInvalid("payment id and order number are required")
	}
	amount, converted, err := xnoAmount(cfg, input.Amount.Decimal, input.Currency)
	if err != nil {
		return nil, err
	}
	minor := amount.Shift(xnoDecimals)
	if !minor.IsInteger() || !minor.IsPositive() {
		return nil, configInvalid("amount %s XNO is not a positive multiple of 0.01", amount.String())
	}
	expected, err := nano.ExpectedRaw(minor.IntPart(), xnoDecimals, fmt.Sprintf("%d:%s", input.PaymentID, orderNo))
	if err != nil {
		return nil, configInvalid("%v", err)
	}
	uri := "nano:" + cfg.Account + "?amount=" + expected.String()
	payload := jsonmap.JSON{
		"nano_account":    cfg.Account,
		"nano_amount_raw": expected.String(),
		"nano_uri":        uri,
	}
	if converted {
		payload["exchange_rate"] = cfg.ExchangeRate
		payload["original_amount"] = input.Amount.Decimal.String()
		payload["original_currency"] = strings.ToUpper(strings.TrimSpace(input.Currency))
	}
	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  expected.String(),
		RedirectURL:  uri,
		QRCodeURL:    uri,
		Payload:      payload,
		AmountSent:   amount.StringFixed(xnoDecimals),
		CurrencySent: CurrencyXNO,
	}, nil
}

// QueryPayment 实现 paymentcontract.GatewayCapturer。providerRef 是 CreatePayment 给出的 raw 金额。
// 只有“已确认的 state send、收款方是本渠道地址、金额正好等于该 raw”才算支付成功；
// 区块哈希随后以 O_EXCL 写入 claim_dir，同一区块不能再给另一笔支付入账，进程重启后仍然成立。
func (a *nanoAdapter) QueryPayment(ctx context.Context, raw jsonmap.JSON, providerRef string) (*paymentcontract.GatewayQueryResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	want, err := nano.ParseRaw(providerRef)
	if err != nil || want.Sign() <= 0 {
		return nil, fmt.Errorf("%w: nano provider_ref %q", paymentcontract.ErrGatewayResponseInvalid, providerRef)
	}
	ref := want.String()
	ctx, cancel := gatewaycommon.WithDefaultTimeout(ctx)
	defer cancel()

	node := nano.Node{URL: cfg.NodeURL}
	hash, err := node.FindSend(ctx, cfg.Account, want, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	}
	if hash == "" {
		return &paymentcontract.GatewayQueryResult{ProviderRef: ref, Status: constants.PaymentStatusPending}, nil
	}
	info, err := node.BlockInfo(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	}
	paid, err := nano.CheckSend(hash, info, cfg.Account, want)
	if errors.Is(err, nano.ErrNotSend) {
		// 区块还未确认：保持 pending，下次查询再看。
		return &paymentcontract.GatewayQueryResult{ProviderRef: ref, Status: constants.PaymentStatusPending}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	}
	if _, err := (nano.FileClaims{Dir: cfg.ClaimDir}).Claim(paid.Hash, ref); err != nil {
		if errors.Is(err, nano.ErrReplayed) {
			return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
		}
		return nil, fmt.Errorf("%w: nano claim: %v", paymentcontract.ErrGatewayRequestFailed, err)
	}
	minor, err := nano.MinorOf(paid.Amount, xnoDecimals)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	}
	return &paymentcontract.GatewayQueryResult{
		ProviderRef: ref,
		Status:      constants.PaymentStatusSuccess,
		Amount:      money.FromDecimal(decimal.NewFromBigInt(big.NewInt(minor), -xnoDecimals)),
		Currency:    CurrencyXNO,
		Payload: jsonmap.JSON{
			"nano_block_hash": paid.Hash,
			"nano_payer":      paid.Payer,
			"nano_amount_raw": paid.Amount.String(),
		},
	}, nil
}
