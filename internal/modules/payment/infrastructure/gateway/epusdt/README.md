# Fixed Epusdt payment intents

Current local implementation (2026-10-09). Merchant-owned pricing, durable recovery, two-source verification, unique consumption, same-origin member cashier, SQL effects/freeze and historical subscriptions are implemented. Production remains disabled; real RPC, browser and deployment acceptance are still required. See [control scope](../../../../../../ops/maimen/payment-automation-control.md).

## Opt-in configuration

`fixed_payment_intent: true` requires `order_mode: "transaction"`, `token: "usdt"`, `network: "tron"` or `"binance"`, and fiat `currency: "cny"` or `"usd"`. The business payment currency must match; changing a currency label is not conversion. The dedicated secure gateway accepts CNY only, requires an empty `payment_type`, a configured `service_secret`, four-decimal precision and a maximum 900-second window; the broader legacy adapter options are not deployment alternatives. Legacy configurations retain their existing behavior and **are not a secure fallback**. Setting a quote policy without enabling fixed mode is rejected.

The merchant channel's `quote_policy` object requires:

| Field | Meaning |
| --- | --- |
| `version` | Nonempty operator-controlled pricing revision, at most 64 characters. |
| `rate` | Positive plain decimal string: **USDT per one unit of the fiat currency**. No exponent notation or automatic external price source. |
| `precision` | Explicit display/allocation precision, 2–6 decimals. |
| `max_tail` | Nonnegative plain decimal string; multiple of the precision step, at most 99 steps. Zero permits only the base amount. |
| `lifetime_seconds` | Merchant authorization's maximum lifetime, 60–86400 seconds, further capped by the business order deadline. |
| `recipient` | One merchant-authorized address for this channel/network. TRON Base58Check is verified; EVM addresses are normalized to lowercase. |

These values are not configured automatically. Production pricing, precision and tail policy still need to be chosen before activation. A fixed rate is merchant-controlled configuration, not a live market feed with freshness guarantees. Policy changes affect new authorizations; existing ones retain their original policy and destination. The SQL automation control is separate from channel pricing. It does not revoke funds already transferred or messages already admitted.

The token contracts are pinned in `payment_intent.go`: mainnet TRON USDT at 6 decimals and BSC's Binance-Peg USDT at 18 decimals. A returned contract is never accepted just because the gateway labels it USDT. Pinning a contract does not verify which chain an RPC is serving; the separate independent verifier also checks chain identity and finality.

## Authorization and binding

1. The application inspects `RequiresPaymentIntent` and locks the SQL control before order/wallet locks. It rejects upstream/unknown item types for new fixed requests, then calls the adapter's local `PreparePaymentIntent` through the contract port. In the same merchant transaction that creates the payment it saves `payments.payment_intent`, a stable gateway order number and `requires_chain_verification=true` **before HTTP**. The snapshot contains no API secret.
2. The merchant computes `round_half_up(fiat_amount × rate, precision)` using decimal arithmetic, then stores integer base amount, permitted maximum and step. GMPay still uses a float fiat field: a non-round-tripping amount is refused, never silently rounded. Fiat amounts must fit the existing two-decimal payment model and GMPay's greater-than-0.01 minimum. No new per-order/day business cap is imposed.
3. The request includes signed `fixed_payment_intent=true` and the original absolute `payment_expires_at_ms`. Gateway terms must match order, network, token, pinned contract/decimals, authorized recipient, fiat facts and rate. The integer amount must be within the authorized base/tail interval and aligned to its step. The envelope must agree with its terms; an error status, already-paid result, malformed/missing terms or conflicting amount is refused.
4. Gateway creation must be no earlier than merchant authorization and no later than local response verification. Expiry must be in the future and no later than the merchant/order deadline. Keep host clocks aligned. The gateway uses the signed merchant deadline rather than recalculating a new duration after network latency; deadlines beyond its 15-minute maximum or already expired are rejected. Idempotent retry cannot extend the original deadline.
5. Terms bind transactionally only to the authorization and automation revision used for the request. A freeze/resume cycle invalidates an in-flight response; subsequent recovery must obtain a current revision. A conflicting competing result cannot replace a bound snapshot. The verified gateway expiry is stored in `payments.expired_at`; repositories can reuse an unexpired fixed payment without a gateway URL. An already bound adapter retry returns the saved terms without another HTTP request.

A retry with the saved authorization keeps its price, request metadata and expiry cap. Fixed create errors retain the initiated payment and wallet reservation. A durable `intent_retry_at` lease drives bounded periodic recovery with the original payment and gateway order ID. Active fixed intents block switching channels/wallet splits; cancellation/expiry and already-bound checks prevent stale binding. The worker restores quote binding only, never payment success.

`payment_intent` is distinct from `provider_payload`, and is excluded from generic payment JSON. Gateway raw responses and gateway-hosted payment/QR URLs are withheld in fixed mode. The member-only merchant cashier endpoint/UI is now implemented with a separate default-disabled flag. Create/latest responses clear all legacy presentation fields in fixed mode, including values extracted from provider payloads. This intermediate version still lacks complete deployment acceptance: do not activate a production channel.

Ordinary callbacks are refused for `requires_chain_verification` payments, including when their snapshot is missing. The transaction rechecks this marker. There is no bypass switch to mark these payments paid. The separate settlement coordinator connects fresh independently verified facts to unique consumption and atomic settlement; removing the callback guard merely because a callback is signed remains unsafe.

Service requests with `service_secret` use the original business HMAC plus a versioned MAC over Host/method/path/direction/time/nonce/raw body. Dedicated HTTPS transport refuses redirects/proxies/private DNS targets and bounds responses. The gateway persists the nonce before work. Merchant main-DB allocations complement retained gateway history and the 100-attempt allocator. Never delete history or reuse slots after expiry; restore both sides coherently and reconcile the backup gap.

## Migration and verification

`Payment` adds `payment_intent` JSON, non-null `requires_chain_verification` (default false), and nullable `intent_retry_at` through the existing GORM migration. The candidate/recovery module also registers two merchant inbox tables. Legacy rows are not backfilled with invented authorization. A code rollback that ignores the marker can reintroduce unsafe callbacks: keep U payments stopped until compatible code/data are restored. No production migration has been run.

From the merchant repository, run:

```sh
go test ./internal/modules/payment/... ./internal/architecture
go test -race ./internal/modules/payment/application ./internal/modules/payment/infrastructure/gateway/epusdt -run 'Test(EpusdtFixedIntent|EpusdtIntentBinding|FixedIntent|EpusdtTransport)' -count=1
```

Tests use synthetic addresses, temporary SQLite databases and local HTTP fixtures. They cover both networks, malicious/missing terms, precision/tail bounds, policy changes, persistence before HTTP, conflicting binds, expiry reuse, callback rejection and transport limits. Additional full merchant tests (196 packages) and PostgreSQL/Redis integration tests passed. Two-network cross-repository synthetic HTTP creation/retry/deadline tests passed. These do not prove real-chain receipt/finality, real TLS/browser behavior or live customer fulfillment.
