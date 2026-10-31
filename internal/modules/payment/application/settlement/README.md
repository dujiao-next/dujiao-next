# Independent fixed-payment settlement

The merchant now contains an opt-in settlement coordinator and durable effect outbox. **Production acceptance is incomplete.** The verifier and settlement flags in [the configuration example](../../../../../ops/maimen/payment-chain-verification.example.yml) default to false. Ordinary fixed-payment callbacks remain blocked; there is no callback bypass to enable this flow.

## Ownership and transaction boundary

[Models](../../domain/settlement.go) and [ports](../../contract/settlement.go) define four persistent tables:

| Table | Invariant |
| --- | --- |
| `payment_allocations` | Unique network/contract/recipient/raw-amount slot and payment ID; expiry and soft deletion do not release ownership. |
| `payment_event_consumptions` | Unique canonical event key, payment, root order, evidence and allocation IDs. |
| `payment_effects` | Separate durable task per payment/order/responsibility, with token lease, last dispatch admission and completion state. |
| `payment_automation_control` | Default-frozen singleton, revision-checked transaction fence; retained through restarts/migrations. |

Before outbound creation, `payments.settlement_basis` freezes merchant economic/delivery obligations. The accepted terms and allocation commit together. An allocation snapshot binds those obligations, payment identity and saved intent. No old authorization or consumption is invented by migrations. The historical-gap check audits every fixed payment without allocation in bounded batches, including expired/deleted rows. Only fully reconstructible unbound authorization with a valid basis and no terms key/provider reference is tolerated. Missing/malformed authorization, terms without a reference, explicit null terms and incomplete economic basis block new allocations; they are never backfilled by guessing. Hard-deleted history or an inconsistent restore cannot be repaired by these constraints.

The [coordinator](service.go) re-verifies a leased candidate using current server-owned RPC policy outside SQL locks. A saved `chain_checked` state or old proof is never a spending credential. Inside the transaction it rechecks receipt conflicts, lease identity, merchant authorization, allocation snapshot, member/root-order and child lifecycle, and exact wallet/online economic split. The existing checkout writes `OnlinePaidAmount` as an expected share before payment; pending orders may contain zero or exactly `TotalAmount - WalletPaidAmount`, never an arbitrary nonzero share. This field alone never proves receipt; fresh chain evidence and unique consumption remain mandatory. Proof must be fresh, inside the original inclusion window and no earlier than the merchant's actual allocation timestamp. Local clock/block timestamp granularity can conservatively send boundary cases to review; do not silently relax this rule.

Event consumption, payment success, order/inventory changes, member spend, configured reseller accounting and outbox creation commit together. Failure rolls everything back. Duplicate/cancelled/expired/ambiguous/out-of-window cases grant no money or goods and do not reopen orders. Historical in-window inclusion can settle only while the business order remains payable. Payment `paid_at` records settlement time; the consumption proof retains chain inclusion facts.

## Durable effects and current limits

`payment:chain_settle` is scheduled every minute when `payment_chain_verification.settlement_enabled` is explicitly enabled, requires the independent verifier and processes at most five candidates with a four-minute task timeout. `payment:effects` processes at most twenty separately leased tasks per run with a two-minute task timeout. SQL leases last two minutes; RPC calls do not hold the settlement transaction. Disabling new settlement does **not** freeze effects already committed. The new [SQL control and operator procedure](../../../../../ops/maimen/payment-automation-control.md) separately fence fixed creation/recovery, settlement, commission, fulfillment and external-send admission; control defaults frozen. The freeze commit waits for previously admitted guarded SQL transactions, and a changed revision invalidates an in-flight settlement proof.

- Member spend is incremented in settlement. The fixed-payment level effect prepares policy outside SQL, then locks the control/order/payment, rereads the user and updates the level plus effect ACK in one transaction. CAS conflicts retry; freeze/resume invalidates old work. Refund/cancellation skips obsolete upgrading. It never increments spend again. Other member-level entrypoints retain their existing behavior; policy administration is not globally serialized with preparation.
- Affiliate commission creation uses the caller's transaction, locked current order, allocation snapshot and an atomic effect completion marker. Refund/cancellation checks prevent creating commission after a closed/refunded order.
- Automatic card delivery is acknowledged only after a delivered fulfillment row exists and the order is completed. Queue acceptance alone leaves the effect pending. A lost Redis task can be re-enqueued; the fulfillment transaction rechecks order state, parent identity, refund and item types before consuming cards. A shared transaction guard detects fixed payments even on the legacy queue branch or manual fulfillment endpoint, checks the control row, unique consumption and current allocation snapshot, and leaves fixed-payment delivery notifications to pre-created outbox tasks.
- Email/Bot/notification/downstream effects call synchronous senders. SQL completion follows sender protocol acknowledgment or explicit policy skip. Configuration/temporary failures retain retry state; permanent rejection and obsolete success notifications after refund/cancellation move to manual review. Each actual request commits control-revision/token/lease admission first; `dispatched_at` is that admission timestamp, not delivery proof. Fixed success/delivery tasks already in Redis are suppressed by database ownership; refund/cancellation notifications retain their original workflow.
- External delivery remains at least once: remote success before local ACK can still duplicate a message. Notification-center fan-out now persists each recipient ACK, so another target failure or parent-effect replay skips already confirmed destinations. Single-recipient order email/Bot/downstream effects retain their parent ACK. No receiver-side idempotency protocol is claimed. SMTP/HTTP acknowledgment does not prove human reading or final Bot delivery. A pre-freeze admitted request can finish afterwards; no SQL lock crosses the network.
- Notification-center recipient admission checks the gate/revision/effect identity/token/unexpired lease, while acknowledgment may record already admitted IO after freeze. Refund/lifecycle reads before external sends still are not atomic with those requests.
- New fixed-payment creation rejects upstream/unknown fulfillment types before reserving funds or requesting a quote. Previously saved upstream procurement items still enter manual review (`upstream_delivery_not_ready`). Their external purchase idempotency is not verified, and this implementation does not automatically retry uncertain purchases.

The [merchant cashier](../cashier/README.md) covers member eligibility and trusted same-origin display. Default-disabled [alerts](../securityalerts/README.md), OS/DB-authorized operator audit, historical source subscriptions, complete TRON receipt / finalized BSC discovery and isolated paired restore tooling are implemented. Local PostgreSQL/Redis recovery tests passed. Live RPC, browser/Access, RN topology and real payment acceptance remain open. Receiver-side message idempotency and upstream procurement are outside this release; new fixed upstream orders are refused. Permanent amount slots still require capacity planning; another merchant must use isolated collection ownership.

## Recipient confirmation

[RecipientBatch](recipients.go) freezes the complete deduplicated destination set before sending; at most 100 unique targets are accepted. `payment_effects.recipient_plan` and `payment_recipient_receipts` are in the merchant database. Keys are digests of channel/recipient (plus Feishu application/ID type), with no recipient address or message body in this receipt ledger. Destination-set change, missing receipts or invalid state needs manual review; do not clear the plan/history to silently reroute an unfinished batch. A disabled scene is an explicit empty plan and cannot erase a previously nonempty plan.

This freezes recipients, not rendered message content. Unconfirmed targets use current template/order facts on retry. A confirmed target is skipped even if the template later changes. Existing notification content logs retain their previous behavior and privacy requirements. Failed ACK leaves the outcome uncertain and retryable; only a recorded protocol acknowledgment suppresses the destination. [Fault tests](../../../../persistence/integrationtest/payment/delivery_safety_test.go) cover partial failure, actual DB reopen, plan change, concurrent admission, freeze after IO and SQL ACK failure.

## Snapshot normalization and validation

`OrderBasis` normalizes persistence-only item timestamps, nonpersistent promotion display names and equivalent empty JSON/tag values before hashing. PostgreSQL stores microsecond timestamps, whereas in-memory objects may contain nanoseconds; this must not falsely invalidate an unchanged authorization. Economic/delivery item contents, SKU, quantity and amounts remain part of the basis and changing them still fails authorization checks.

Cross-module tests now live in [payment integration tests](../../../../persistence/integrationtest/payment/). They were moved out of this application layer because they use global migration/container/Redis adapters; no architecture rule was relaxed. Run from the merchant root:

```sh
go test ./internal/modules/payment/... ./internal/persistence/integrationtest/payment ./internal/architecture
go test -race -count=1 ./internal/persistence/integrationtest/payment ./internal/modules/payment/infrastructure/gormstore/automation
# Only with dedicated synthetic PostgreSQL / Redis, never production credentials:
# PAYMENT_TEST_POSTGRES_URL=... PAYMENT_TEST_REDIS_HOST=... go test -count=1 -run TestPostgresPayment ./internal/persistence/integrationtest/payment
```

Local tests passed real PostgreSQL16.14 / Redis7.4.9 migrations, concurrent settlement and cross-order replay, outbox-write rollback, freeze lock ordering, lost Redis tasks and registered consumer delivery with one synthetic card/fulfillment. SQLite tests additionally cover restart, missing history, allocation conflicts, effect leases, recipients, notification ACKs and refunds. RPC/SMTP/HTTP inputs remain synthetic. No real-chain, browser, production migration or customer payment acceptance is implied.
