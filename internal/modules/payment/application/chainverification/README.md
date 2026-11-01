# Independent USDT evidence verification

A separate, default-disabled [security alert service](../securityalerts/README.md) now observes persisted manual/backlog/RPC-retry conditions. It does not perform RPC probes or certify head/scanner liveness; actual provider and heartbeat acceptance remains required.

The [merchant cashier](../cashier/README.md) now reads validated instructions from merchant storage. Its `cashier_enabled` flag is separate and default-disabled; bootstrap also requires verifier and settlement enabled. Display readiness does not prove live RPC health or authorize settlement.

The [SQL payment automation fence](../../../../../ops/maimen/payment-automation-control.md) intentionally leaves this fact-checking worker running. It gates creation/recovery, payment/delivery transactions and actual notification-send admission in their separate workflows; disabling this verifier alone is not a delivery freeze.

This worker checks chain evidence and stores a normalized result. **It does not consume a transfer, pay an order, refund/credit a wallet or trigger delivery.** A separate [settlement coordinator](../settlement/README.md) now implements allocation, fresh re-verification, unique consumption and core settlement/outbox; its local atomicity and downstream queue recovery have passed synthetic PostgreSQL/Redis tests; live acceptance remains incomplete. Production U payments remain disabled.

## Trust boundary and configuration

`payment_chain_verification` is server-owned configuration in `internal/config/config.go`, not payment-channel JSON and not data from Epusdt. The default is disabled. The [example configuration](../../../../../ops/maimen/payment-chain-verification.example.yml) has no live credentials or valid anchors and must not be enabled as-is. Existing private configuration was not modified this batch.

Each network requires exactly two sources with distinct operator-supplied provider labels, names and hostnames. Operations must verify they are actually independent: different labels/URLs do not prove different operators, and Alchemy HTTP/WSS are one provider. Both sources must produce the same transaction/event/block/amount/destination facts; neither unavailable sources nor malformed responses permit single-source fallback. BSC checks chain ID 56 and a pinned mainnet block. TRON checks a pinned mainnet solid block (including block ID's height prefix). Choose anchors from a separately reviewed mainnet record; never bootstrap them from a gateway response or blindly trust a newly configured endpoint. Anchor lookup must be supported by both providers.

The policy needs a nonempty version and a maximum solid/finalized head age from 30 to 600 seconds. Updating a source, anchor or policy requires a reviewed config change/restart and a new policy version. Proof stores provider labels, policy version, anchor and solid/finalized head facts; no RPC URL or API key is stored in it. This is independent RPC agreement, **not a self-validating consensus/light-client proof**. Simultaneous compromise/collusion of the trusted sources remains outside its guarantee.

The [RPC client](../../infrastructure/chainrpc/http.go) accepts only configured HTTPS destinations on the default HTTPS port, verifies TLS (minimum 1.2), refuses redirects/proxy environment variables, rejects credentials/query strings in URLs and resolves/checks public IPs at connection time before dialing those exact IPs. Loopback/private/link-local/metadata and selected special ranges are denied, including mapped IPv4. It does not use an `allow_private` fallback. This restriction applies to the new verifier, not all historical gateway HTTP paths.

Requests have a 10-second HTTP timeout, 7-second header timeout, 4 MiB response bound and a per-provider/process limiter of 2 requests/second with burst 1. An entire candidate verification has a 45-second deadline. Multiple worker processes multiply this limit; real quota/capability acceptance is still required. Errors are fixed summaries that do not expose credential-bearing URLs or server error bodies.

## Required evidence

The [saved intent reader](../../infrastructure/gateway/epusdt/payment_intent.go) reconstructs the merchant's original quote authorization and checks its bound terms, payment/order/trade identity, pinned mainnet contract/decimals, recipient, raw integer amount, rate/tail and window. It does not read current channel pricing, extend expiry or reject a valid historical intent merely because verification runs after expiry. Ordinary-callback guards remain in place, including for missing authorization.

Only the candidate transaction hash is an RPC lookup hint after local identity checks. Gateway amount, address, time, event index, review reason and `Removed` cannot authorize a payment. The worker finds the actual matching receipt event itself.

| Network | Required checks |
| --- | --- |
| BSC | `eth_chainId=56`; pinned block; successful receipt; matching transaction, receipt and canonical block hash/height/index; transaction inclusion in block; exact pinned USDT Transfer topic, padded addresses, 256-bit amount and unique log index; explicit `removed=false`; exactly one matching event; fresh `finalized` head covers the inclusion block; canonical inclusion block re-read after finality lookup. |
| TRON | Pinned block through `walletsolidity`; transaction ID equals SHA-256 of `raw_data_hex`; explicit transaction and VM `SUCCESS`; matching transaction-info ID, block number/time; transaction inclusion in the solid block; actual pinned USDT Transfer log, padded addresses and exact amount; exactly one matching event; fresh solid head covers the block; inclusion block re-read. Uses solidity endpoints, not calldata as proof. |

A solid/finalized head at the same height as the inclusion block must have the same block hash and timestamp; its height must also cover the trusted anchor. Missing `finalized` support never falls back to `latest` or an assumed confirmation count. Missing transaction/receipt finality retries; malformed success evidence, mismatched economic facts and ambiguous matches do not pass. The receipt parser rejects mint-like zero senders and any pinned-token outgoing event from the receiving address in the same transaction, including self-transfers. Multiple matching transfers require manual review. TRON indirect calls/transferFrom can be verified if supplied as a candidate, but the gateway's discovery coverage is still limited.

Inclusion must be in `[terms.created_at_ms, terms.expires_at_ms)`. A transfer at the deadline or outside that window goes to manual review. A transfer in the window can be independently checked later; a cancelled/expired business order is not reopened by this worker. Finality is only as credible as the configured sources and checks.

## Durable workflow

The separate `payment:chain_verify` task is registered every minute only when enabled. It has a four-minute task timeout, no Asynq immediate retries and processes up to five candidates, claiming one just before each verification. Existing candidate ingestion/create recovery remain separate tasks so a slow RPC does not hold ACK reception hostage.

`payment_candidate_evidence` adds `check_state`, `check_code`, `check_after`, `check_token`, `check_attempts`, `chain_proof` and indexed `event_key` through the existing central AutoMigrate. Existing observations start pending; no proof is invented for old rows. SQL leases last two minutes; a new random token prevents an expired worker from overwriting a reclaimed attempt. No HTTP call holds the SQL transaction. Failure to persist leaves the lease recoverable.

| State | Meaning |
| --- | --- |
| `pending` | Received evidence awaiting its first check. |
| `checking` | A worker owns a temporary SQL lease. |
| `retry` | Binding/finality/source availability/disagreement needs another attempt; backoff grows from 1 to 10 minutes. |
| `manual` | Identity/evidence mismatch, out-of-window, ambiguous events or conflicting candidate versions. No automatic retry or financial action. |
| `chain_checked` | Two configured sources agreed on normalized transfer facts. **No ownership or settlement permission is implied.** |

Finish rechecks the lease, receipt conflict flag and current authorization/payment identity. Changed authorization retries instead of saving stale proof. A later different payload under the same gateway source/candidate ID marks all versions manual and revokes any earlier checked state; historical proof may remain for investigation. Consumers must never treat a nonempty `chain_proof`/`event_key` alone as approval.

`event_key` hashes network/transaction hash/receipt event index, excluding block hash and order ID, so reorgs or order aliases cannot manufacture another event identity. **This column is currently a non-unique lookup index, not a consumption ledger.** The separate settlement coordinator now enforces uniqueness in `payment_event_consumptions` and ownership in `payment_allocations`, together with payment/order/outbox changes, lifecycle/conflict checks and fresh verification. The evidence index itself intentionally stays non-unique.

## Acceptance and maintenance

Tests use synthetic addresses, local fake RPCs and SQLite. They cover positive in-window transfers verified after expiry; wrong chain/anchor/token/recipient/amount, failed/missing success, absent or ambiguous Transfer, removal/reorg, stale/future finality, unsupported finality, provider disagreement, self-transfers, transport limits/redaction, database close/reopen, lease takeover/concurrency, changed authorization and conflict invalidation. Schema tests preserve pre-existing received evidence without inventing verification.

```sh
go test ./internal/modules/payment/... ./internal/architecture ./internal/app/jobs/... ./internal/app/container ./internal/bootstrap/database/migrations
go test -race ./internal/modules/payment/infrastructure/chainrpc ./internal/modules/payment/application/chainverification ./internal/modules/payment/application/candidates ./internal/modules/payment/infrastructure/gateway/epusdt ./internal/modules/payment/infrastructure/gormstore -run 'Test(BSCIndependent|TRONIndependent|RPC|ReadChainExpectation|Chain|Candidate)' -count=1
```

Not yet accepted against live providers: TRON second-source independence and budget, BSC finalized support, anchors, quotas/latency and TLS/DNS integration. Local PostgreSQL locking, Redis lost-task recovery and synthetic delivery have passed in `internal/persistence/integrationtest/payment`; these are not real-chain/browser acceptance. No RN migration, real payment or Telegram message occurred. Preserve evidence and allocations in paired manual backups; restore frozen and reconcile before replay/resume. Keep both verifier/settlement and cashier activation flags default-disabled.
