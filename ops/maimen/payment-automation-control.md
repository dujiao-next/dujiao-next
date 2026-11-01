# Fixed-payment operator and SQL fence

Current implementation: 2026-10-09, local acceptance only; no production migration or deployment. See the companion gateway `ops/secure/README.md` for paired snapshots. Keep all three configuration switches disabled until deployment acceptance.

## Persistent control

`payment_automation_control` contains the singleton row `id=1`. Central migration
creates it **frozen**, revision 1. Rerunning migration preserves existing state.
A missing row, failed database query, or frozen row never authorizes a guarded operation.
The verifier, settlement and cashier configuration flags still default to false. Opening
the database fence does not enable a disabled settlement service.

The [SQL adapter](../../internal/modules/payment/infrastructure/gormstore/automation/control.go)
provides `SetFrozen` with an expected revision and a reason. The local `cmd/payment-ops` operator writes `payment_operator_audit` in the same transaction as freeze/resume. No callback, public HTTP route or Telegram command exposes it. No startup automatically thaws the fence.

## Transaction ordering and coverage

The conditional control-row write is held until the **same transaction** commits
or rolls back. It works with SQLite's write lock as well as PostgreSQL row locks;
the implementation does not rely on SQLite honoring `FOR UPDATE`. Lock ordering:
control → receipt/evidence (settlement only) → root/children → payment → effects/cards.
RPC and outbound notification calls do not hold this lock.

| Operation | While frozen |
| --- | --- |
| Merchant cashier read | Returns paused without address/amount; no new ready response admission. Previously admitted responses/copies cannot be revoked. |
| New settlement leases | Not claimed. |
| Settlement already verifying RPC | Commit refused; after lease expiry and resume, a new verification is required. A freeze/resume revision change also invalidates the old proof. |
| New payment-effect leases | Not claimed. Preleased handlers also check the current revision before dispatch. |
| Affiliate commission SQL commit | Checks the fence within the commission/ack transaction. |
| Fixed-payment automatic card delivery | Refused inside the delivery transaction, including an old queue payload without its durable-payment marker. |
| Fixed-payment manual fulfillment endpoint | Refused by the same transaction guard. It is not a freeze bypass. |
| Ordinary non-fixed payment fulfillment | Continues; the fixed-payment control is not a site-wide shutdown. |
| Candidate receipt/ACK and independent verification | Continue retaining evidence. These operations do not grant payment or delivery. |
| Fixed payment creation/reuse/recovery | Creation and local preparation take the control lock before order/wallet locks; frozen recovery does not start a new request. Binding rechecks the original request revision, so freeze/resume invalidates an old HTTP response. Already authorized outbound requests may finish; their late result cannot bypass binding checks. |
| Fixed payment email/Bot/notification/downstream sends | Each actual send first commits SQL admission against the current revision, identity, token and unexpired lease. No new admission while frozen. An already admitted request may finish afterwards. |
| Fixed-payment member-level effect | Level update and effect ACK share a transaction with control revision, order/payment/allocation and lease checks. Other level entrypoints keep their existing behavior. |
| Security alerts | Continue observations and outbound alerts independently; recording an already admitted notification ACK also remains allowed. |
| Refund/cancellation notifications | Keep their existing workflow; this payment-success outbox does not own them or promise their durable recovery. |

Successful freeze commit waits for an already admitted guarded SQL transaction.
That transaction may have completed payment or delivery **before** the freeze.
Inventory already consumed, secrets already delivered, or messages already sent
cannot be recalled. This global fixed-payment fence is not a per-network switch.

The delivery adapter also rechecks unique consumption, the payment/allocation
identity and the current root/child economic snapshot before consuming cards.
Changing order goods after payment does not authorize a different delivery.
Fixed manual and automatic deliveries both leave their notifications to the
pre-created payment outbox, avoiding a second legacy notification dispatch.

## Notification acknowledgment and creation limits

Fixed payment messages use synchronous senders owned by the SQL outbox, not a
second Redis notification queue. Temporary send/configuration failures retain
pending work with a 1–15 minute backoff. Explicit policy skips can complete;
permanent Bot rejection or invalid email targets move to manual review without
a successful completion timestamp. Success notifications found after refund or
cancellation also move to review. This is a pre-send lifecycle read, not an atomic
transaction with a concurrent refund or an external recipient.

`payment_effects.dispatched_at` records the last committed admission, **not delivery**.
Completion means the SMTP/HTTP sender acknowledged the request (or policy skipped
it), not that a person read a message or the remote Bot delivered to Telegram.
A crash after sending but before SQL acknowledgment can cause duplicates. The
notification center now persists recipient ACKs and freezes the full target set,
so another target failure or parent replay does not resend confirmed recipients.
Target changes or missing receipt history require review. Single-target order
email/Bot/downstream effects retain parent-effect acknowledgment. Receivers
needing stronger guarantees still need their own durable idempotency protocol.
Old queued payment/delivery messages are suppressed using database ownership;
refund notifications remain on their existing path.

SMTP connections have a 10-second dial timeout and 30-second protocol deadline;
the legacy sender interface cannot interrupt an admitted SMTP call immediately
on cancellation. Notification admission is checked again for each recipient.
A frozen queue still needs its scheduler/process restarted before pending work
can progress; SQL persistence does not make Redis/process availability automatic.

Fixed payment creation/recovery now shares persisted member order/item eligibility
with the [merchant cashier](../../internal/modules/payment/application/cashier/README.md):
empty/inconsistent structures, upstream/unknown types and invalid lifecycle/expiry
are rejected. Trusted address/amount display is implemented in the existing
same-origin member page. The pay host is service-only. Browser and real operator-access acceptance remain deployment work; separate per-network settlement controls are not implemented.
Never expose a gateway-hosted cashier URL as a substitute.


## Local operator command

Build `go build ./cmd/payment-ops`; the fullstack Dockerfile also installs `/app/payment-ops`. It reads the same config.yml / database as the app, from the normal config search paths. Run in the app working directory with its private config mount. It starts no HTTP server, worker or migration. Confirm the target DB before any change.

```sh
/app/payment-ops -action status
/app/payment-ops -action freeze -revision 1 -operator operator-id -reason incident-reference
/app/payment-ops -action status
# Only after reconciliation and acceptance, using the CURRENT returned revision:
/app/payment-ops -action resume -revision 2 -operator operator-id -reason reviewed-recovery-reference
/app/payment-ops -action note -payment 123 -operator operator-id -reason manual-investigation-reference
```

Revision numbers and payment IDs above are examples, never blindly reusable. Freeze/resume require a current expected revision; stale revision or audit-write failure rolls back the change. `note` only writes an audit record for an existing fixed payment: it does not settle, deliver, refund, edit amounts or delete evidence. Read records in `payment_operator_audit` using a read-only DB operator session; this is not a public management API.

Authentication is the existing OS / Docker / database permission boundary. The `-operator` string is an accountability label, not cryptographically verified identity; restrict sudo / Docker / DB access and preserve host audit logs. Do not put secrets or customer data in reasons. Same-DB audit is not tamper-proof against DB administrators. Global freeze covers both fixed U-payment networks; disabling one channel does not freeze its old orders.

## Backup and restore order

Freeze and record returned revision → stop gateway → take consistent merchant primary DB snapshot → gateway SQLite backup / manifest → manual Mac fetch / age encryption. Keep API/worker stopped when restoring the merchant; run operator freeze on the restored DB first. Restore gateway into a new isolated directory, retain current revocations and config, clear old sessions, replay candidate ACKs. Reconcile the backup gap before resuming: chain history cannot reconstruct already delivered goods.

## Validation and limits

The full merchant suite passed 196 test packages with Go1.26.9. Integration tests live in `internal/persistence/integrationtest/payment` (moved from application/settlement to honor architecture rules). Synthetic PostgreSQL16.14 / Redis7.4.9 tests cover real migrations, concurrent settlement, unique event consumption, transaction rollback on outbox failure, SQL freeze locking, lost Redis tasks, requeue and registered consumer delivery. SQLite/race tests cover receipts, effects, operator audit and revision conflicts.

No real RPC, production migration, live notification or customer payment acceptance is implied. Refund/cancellation notifications retain their existing workflow. Receiver-side idempotency and independent host-down monitoring are not implemented; outbound sends acknowledged remotely before local SQL ACK can still duplicate. Do not loosen the fence to resolve delivery errors.
