# Merchant candidate receipt and create recovery

The [SQL fence](../../../../../ops/maimen/payment-automation-control.md) leaves receipt/ACK ingestion running, but now blocks fixed create/reuse/recovery preparation and checks the original request revision again when binding. A freeze/resume cycle cannot approve an old HTTP response. Recovery after reopening preserves the original identity, quote and wallet reservation. The [merchant cashier](../cashier/README.md) now validates persisted eligibility and exposes a same-origin member-only view; live topology/browser acceptance remains required before launch.

This module does not authorize payment or delivery. A separate [independent verification worker](../chainverification/README.md) now checks RPC evidence and persists its state; a separate [settlement coordinator](../settlement/README.md) now owns allocation and atomic consumption. Local PostgreSQL/Redis settlement and effect tests have passed; live-provider and production acceptance remain deployment gates.

## Durable delivery

The registered `payment:candidate_sync` task runs every minute with a two-minute deadline when the existing worker is enabled. It polls each persisted source once per cycle (up to 100 candidates). New fixed authorizations save `payment_candidate_subscriptions` in the same merchant transaction; current fixed channels also populate subscriptions. Endpoint/PID define identity. Disabled/deleted channels or a changed PID do not erase historical polling. Scan-health and receipt success are persisted for the security monitor; this is gateway-reported health, not an independent host availability probe.

The [adapter](../../infrastructure/gateway/epusdt/candidate.go) signs the list/ACK action and scalar parameters. Candidate IDs stay canonical positive uint64 decimal strings, including values above JavaScript's safe integer range. ACK uses signed CSV IDs. Responses refuse redirects, malformed batches, duplicate IDs and oversized bodies (512 KiB for a candidate list; 64 KiB for create/ACK). Economic observations are still untrusted even if structurally valid.

The [store](../../infrastructure/gormstore/candidate_store.go) commits the complete batch before ACK:

- `payment_candidate_receipts`: unique SHA-256 of the JSON tuple `[normalized gateway_url, pid]` plus remote candidate ID; first evidence digest, receipt time and sticky conflict flag. Secrets and channel IDs are not part of identity.
- `payment_candidate_evidence`: every distinct typed observation version, unique per receipt/digest. Replays do not duplicate evidence; altered content for an existing ID appends a version and flags the receipt. This is not a unique chain-transfer consumption key.
- A local payment ID is only an identity hint. Matching requires exactly one local gateway order identity, fixed-mode marker, Epusdt provider, saved payment/order IDs, original endpoint/PID, and (when bound) both saved trade IDs. Expired/soft-deleted payment history is considered. Nothing in an observation can change `payment_intent`, the wallet, order or payment status.

If SQL fails, no ACK is attempted. If the process stops after SQL commit, or an ACK response is lost, the next list safely repeats pending gateway rows. Gateway review reasons, including late, removed or ambiguous observations, are retained without granting payment authority. Unmatched observations are retained too. Match state is a receipt-time hint: the independent verifier rereads trusted authorization and receipt conflict state instead of treating that hint as approval; the separate settlement transaction additionally enforces lifecycle and event ownership.

## Create timeout recovery

[Payment creation](../payment_service_create.go) commits the payment row, fixed authorization, stable gateway order number, deadline and first retry lease in one transaction before HTTP. Local preparation failures roll back that transaction. After authorization commits, transport or malformed-response failures retain the initiated payment and reserved wallet balance: they do not prove remote creation failed.

While a fixed intent remains active, repeated clicks use that payment. A switch to another channel or wallet-only payment is rejected; changing `use_balance` cannot alter its saved wallet/online split. Wait for expiry/cancellation before making a fresh allocation. Recovery leases prevent immediate retry floods; they are not payment evidence.

After inbox polling, [the recovery method](../payment_service_provider.go) claims up to five due SQL authorizations per cycle. `payments.intent_retry_at` holds a one-minute lease using a conditional SQL update. Each retry preserves original order number, request metadata, rate and recipient even after a pricing-policy edit. Binding clears the retry lease. Expired, superseded and already-bound payments are not claimed; cancelled/expired business orders are not bound. Disabled sales alone does not abandon existing requests. No network call holds the payment database transaction.

The fixed-mode marker and ordinary-callback guards remain mandatory. This worker only recovers quote binding; it never queries gateway success to pay an order. Missing/corrupt authorization fails closed. Historical failed authorizations from older experimental builds are not automatically promoted into the new recovery queue.

## Operations and limits

The central GORM registry adds both inbox tables and `payments.intent_retry_at`. No production migration has been executed. Include the merchant main database, gateway main database and runtime database in manual consistent backups. Inbox evidence is financial recovery material, not log/cache data. Retain gateway ACKed evidence as well: restoring the merchant to a backup older than its ACKs needs a coordinated replay procedure, which is implemented by the gateway isolated restore preparation (reset candidate ACKs without deleting observations), with real deployment recovery still to be accepted.

`payment_candidate_subscriptions.config_json` retains private business/service credentials so old sources remain pollable. It has the same confidentiality level as existing channel configuration; include it only in private encrypted DB backups. Endpoint/PID rotation requires old-source availability or reviewed draining/migration. The secure gateway enables one current PID, so changing it in place without a plan can strand old subscriptions. Do not delete subscriptions or gateway history to silence alerts. Reused IDs after a destructive rebuild become conflicts rather than silently replacing evidence. The gateway secure profile now has constrained egress and a coordinated snapshot/restore procedure; real deployment acceptance remains required.

## Verification

```sh
go test ./internal/modules/payment/... ./internal/architecture ./internal/app/jobs/... ./internal/app/container ./internal/bootstrap/database/migrations
go test -race ./internal/modules/payment/application ./internal/modules/payment/application/candidates ./internal/modules/payment/infrastructure/gateway/epusdt ./internal/modules/payment/infrastructure/gormstore -run 'Test(EpusdtFixed|EpusdtIntentBinding|Candidate|FixedIntentRecovery)' -count=1
```

Synthetic SQLite/local HTTP tests cover disk DB close/reopen, persistence failure and batch rollback, ACK-loss redelivery, conflicting versions, source isolation, 100-row/oversized batches, concurrent writes/leases, dropped create responses, original-request retries after pricing changes, retained wallet funds and cancellation guards. Additional tests in `internal/persistence/integrationtest/payment` passed on PostgreSQL16.14 and Redis7.4.9, including persisted subscription credentials, concurrent settlement and lost-queue redelivery. Real RPC/WSS identity/finality, browser behavior and customer payment acceptance remain unverified.
