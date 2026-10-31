# Payment security alerts — local implementation

The optional `payment_security_alerts` service sends aggregate operator alerts through a separate, outbound-only Telegram sender. It does not change payment state, freeze/thaw controls, refund or deliver goods. No Telegram inbound commands or webhook are added. Live deployment acceptance remains incomplete.

## Configuration and scheduling

See the [private-value-free example](../../../../../ops/maimen/payment-chain-verification.example.yml). `enabled` defaults to false; `bot_token` and numeric `chat_id` must be provided only in private server configuration. Enabling requires the existing worker queue. Container bootstrap constructs the sender without contacting Telegram. No live credentials were read or used during implementation.

The registered `payment:security_alerts` task runs every minute, with a one-minute deadline, uniqueness window and no Asynq retry; SQL retains delivery work for future runs. This service deliberately does **not** consult the payment automation gate before observing/sending. A payment freeze must not silence its own alert. It shares the merchant database and worker infrastructure and is not an external availability monitor.

## Observations

The [SQL store](../../infrastructure/gormstore/securityalerts/store.go) reads only local state and persists all aggregate observations after every query succeeds. Under READ COMMITTED these are operational counts, not a consistent accounting snapshot. A failed query does not produce an empty/healthy recovery report.

| Condition | Observation |
| --- | --- |
| `scanner_unhealthy` | A subscription older than five minutes reports unhealthy scanning or no successful observation in three minutes. |
| `control_missing` | SQL automation row is absent. |
| `automation_frozen` | SQL automation row is frozen. |
| `candidate_conflicts` | Persisted conflicted candidate receipts. |
| `chain_manual` | Candidate evidence in manual state. |
| `rpc_retry` | Evidence waiting over two minutes with RPC-unavailable or disagreement codes. |
| `chain_backlog` | Evidence pending/checking/chain_checked/settling for over five minutes. |
| `intent_stalled` | Fixed initiated/pending payments without provider binding, older than five minutes. |
| `effects_manual` | Payment effects requiring manual review. |
| `effects_stalled` | Paid notification/accounting/auto-fulfillment tasks older than five minutes; delivery notifications waiting for manual fulfillment are excluded. |

These counts do not identify an attacker or prove a chain transaction's validity. Conflict/manual history may remain active and keep reminding until an approved review workflow resolves it; never delete payment history merely to silence an alert. Candidate polling now persists explicit gateway scan-health and subscription success timestamps. This detects reported scanner stalls and polling failures even without candidate backlog, but is not independent RPC truth or an external host-down monitor. The same DB/worker/network failure may prevent the alert itself.

## Durable delivery and privacy

`payment_security_alerts` stores one row per known condition: current count/state, observation version, pending state, lease token, next attempt, attempts and last protocol acknowledgment. It stores no wallet addresses, customer/order contents, raw candidate payload, message body, API credentials or remote error text.

At most three due alerts are leased per run, with a one-minute lease. Failed sends back off from one to fifteen minutes; unchanged active conditions repeat no more often than every fifteen minutes after success. Active/recovered transitions get a new version and an immediate eligible notification; frequent transitions are still bounded by the task cadence and three-message batch. An older token/version cannot acknowledge newer work. Recovery is reported after a prior send or potentially in-flight send, and then stops after acknowledgment. A crash between remote success and SQL acknowledgment may duplicate a message; this is at-least-once delivery.

The [Telegram transport](../../infrastructure/securitytelegram/sender.go) allows only HTTPS `api.telegram.org:443`, resolves/checks and pins public dial addresses, disables environment proxies and redirects, uses TLS 1.2+, and enforces a ten-second request deadline and 64 KiB response bound. It requires HTTP 200, `ok=true`, a positive message ID and the configured chat ID. Errors are fixed summaries; token-bearing request URLs and response descriptions are not returned. Telegram acknowledgment is not proof an operator read the message.

Messages contain only a fixed Chinese condition label, aggregate count and observation version. No Markdown/HTML parse mode, gateway-provided text, arbitrary link or callback control is accepted. This transport hardening applies to this new sender; it is not a claim that every legacy outbound client has been restricted.

## Acceptance and operations

Synthetic SQLite tests cover restart/retry, cooldown, recovery, payment-freeze independence, state/version changes, failed observations and aggregation. A fake HTTP transport covers actual Telegram acknowledgment shape, wrong chat, malformed/oversized replies, 429/redirect/network failure, privacy and unknown-condition rejection. No live Telegram message, RPC or RN operation was performed.

Back up this table with the merchant database and [recipient receipts](../settlement/README.md). Restoring an older backup may repeat old alerts; no receipt history is reconstructed by guessing. Production still needs the real bot/chat permission test, monitoring responsibility, external worker/host/database/network availability checks and scanner/RPC heartbeat acceptance. If this same database, worker, network or Telegram fails, this channel cannot guarantee notification. Logs and SQL state must remain reviewable locally.
