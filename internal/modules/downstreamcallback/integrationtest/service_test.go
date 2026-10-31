package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	downstreamapp "github.com/dujiao-next/internal/modules/downstreamcallback/application"
	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	downstreamdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
)

type refRepositoryStub struct {
	byID      map[uint]*downstreamdomain.OrderRef
	byOrderID map[uint]*downstreamdomain.OrderRef
	created   *downstreamdomain.OrderRef
	updated   []*downstreamdomain.OrderRef
}

func (r *refRepositoryStub) GetByID(id uint) (*downstreamdomain.OrderRef, error) {
	return r.byID[id], nil
}

func (r *refRepositoryStub) GetByOrderID(orderID uint) (*downstreamdomain.OrderRef, error) {
	return r.byOrderID[orderID], nil
}

func (r *refRepositoryStub) GetByCredentialAndDownstreamNo(uint, string) (*downstreamdomain.OrderRef, error) {
	return nil, nil
}

func (r *refRepositoryStub) Create(ref *downstreamdomain.OrderRef) error {
	r.created = ref
	return nil
}

func (r *refRepositoryStub) Update(ref *downstreamdomain.OrderRef) error {
	r.updated = append(r.updated, ref)
	return nil
}

func (r *refRepositoryStub) ListPendingCallbacks(int) ([]downstreamdomain.OrderRef, error) {
	return nil, nil
}

func (r *refRepositoryStub) ListByCredentialID(uint, downstreamcontract.RefListFilter) ([]downstreamdomain.OrderRef, int64, error) {
	return nil, 0, nil
}

type orderReaderStub struct {
	orders map[uint]*downstreamcontract.OrderSnapshot
}

func (r orderReaderStub) GetByID(id uint) (*downstreamcontract.OrderSnapshot, error) {
	return r.orders[id], nil
}

type credentialReaderStub struct {
	credentials map[uint]*downstreamcontract.Credential
}

func (r credentialReaderStub) GetByID(id uint) (*downstreamcontract.Credential, error) {
	return r.credentials[id], nil
}

type queuedCallback struct {
	refID uint
	delay time.Duration
}

type callbackQueueStub struct {
	callbacks []queuedCallback
}

func (q *callbackQueueStub) EnqueueCallback(refID uint, delay time.Duration) error {
	q.callbacks = append(q.callbacks, queuedCallback{refID: refID, delay: delay})
	return nil
}

type delivererStub struct {
	request downstreamcontract.DeliveryRequest
	err     error
	calls   int
}

func (d *delivererStub) Send(_ context.Context, request downstreamcontract.DeliveryRequest) error {
	d.calls++
	d.request = request
	return d.err
}

func newService(references downstreamcontract.Repository, orders downstreamcontract.OrderReader, credentials downstreamcontract.CredentialReader, queue downstreamcontract.CallbackQueue, deliverer downstreamcontract.Deliverer) *downstreamapp.Service {
	return downstreamapp.NewService(downstreamapp.Options{
		References:  references,
		Orders:      orders,
		Credentials: credentials,
		Queue:       queue,
		Deliverer:   deliverer,
		Now:         func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	})
}

func TestCreateRefInitializesPendingStatus(t *testing.T) {
	references := &refRepositoryStub{}
	service := newService(references, orderReaderStub{}, credentialReaderStub{}, nil, &delivererStub{})
	ref := &downstreamdomain.OrderRef{OrderID: 42, CallbackStatus: downstreamdomain.StatusFailed}

	if err := service.CreateRef(ref); err != nil {
		t.Fatalf("CreateRef() error = %v", err)
	}
	if references.created != ref || ref.CallbackStatus != downstreamdomain.StatusPending {
		t.Fatalf("created ref mismatch: %#v", references.created)
	}
	if err := service.CreateRef(nil); !errors.Is(err, downstreamcontract.ErrInvalidRef) {
		t.Fatalf("CreateRef(nil) error = %v", err)
	}
}

func TestEnqueueCallbackResolvesParentAndResetsRetryState(t *testing.T) {
	parentID := uint(17)
	ref := &downstreamdomain.OrderRef{
		ID:                 9,
		OrderID:            parentID,
		CallbackURL:        "https://callback.example.test",
		CallbackStatus:     downstreamdomain.StatusFailed,
		CallbackRetryCount: 4,
	}
	references := &refRepositoryStub{byOrderID: map[uint]*downstreamdomain.OrderRef{parentID: ref}}
	orders := orderReaderStub{orders: map[uint]*downstreamcontract.OrderSnapshot{21: {ID: 21, ParentID: &parentID}}}
	queue := &callbackQueueStub{}
	service := newService(references, orders, credentialReaderStub{}, queue, &delivererStub{})

	service.EnqueueCallback(21)

	if len(queue.callbacks) != 1 || queue.callbacks[0].refID != ref.ID || queue.callbacks[0].delay != 0 {
		t.Fatalf("queued callbacks = %#v", queue.callbacks)
	}
	if ref.CallbackStatus != downstreamdomain.StatusPending || ref.CallbackRetryCount != 0 {
		t.Fatalf("ref was not reset: %#v", ref)
	}
	if len(references.updated) != 1 {
		t.Fatalf("updates = %d, want 1", len(references.updated))
	}
}

func TestSendCallbackBuildsPayloadAndPersistsSentStatus(t *testing.T) {
	ref := &downstreamdomain.OrderRef{
		ID:                3,
		OrderID:           8,
		ApiCredentialID:   5,
		DownstreamOrderNo: "remote-8",
		CallbackURL:       "https://callback.example.test",
		CallbackStatus:    downstreamdomain.StatusPending,
	}
	references := &refRepositoryStub{byID: map[uint]*downstreamdomain.OrderRef{ref.ID: ref}}
	orders := orderReaderStub{orders: map[uint]*downstreamcontract.OrderSnapshot{8: {
		ID:      8,
		OrderNo: "DJ-8",
		Status:  "completed",
	}}}
	credentials := credentialReaderStub{credentials: map[uint]*downstreamcontract.Credential{5: {
		ID:        5,
		APIKey:    "downstream-key",
		APISecret: "downstream-secret",
	}}}
	deliverer := &delivererStub{}
	service := newService(references, orders, credentials, nil, deliverer)

	if err := service.SendCallback(context.Background(), ref.ID); err != nil {
		t.Fatalf("SendCallback() error = %v", err)
	}
	if deliverer.calls != 1 || deliverer.request.Payload.Event != "order.fulfilled" {
		t.Fatalf("delivery request mismatch: %#v", deliverer.request)
	}
	if deliverer.request.APIKey != "downstream-key" || deliverer.request.Payload.Timestamp != 1_700_000_000 {
		t.Fatalf("signed delivery inputs mismatch: %#v", deliverer.request)
	}
	if ref.CallbackStatus != downstreamdomain.StatusSent || ref.LastCallbackAt == nil {
		t.Fatalf("callback result mismatch: %#v", ref)
	}
	if len(references.updated) != 1 {
		t.Fatalf("updates = %d, want 1", len(references.updated))
	}
}

func TestSendCallbackFailureSchedulesBackoffAndPersistsRetry(t *testing.T) {
	ref := &downstreamdomain.OrderRef{ID: 3, OrderID: 8, ApiCredentialID: 5, CallbackURL: "https://callback.example.test"}
	references := &refRepositoryStub{byID: map[uint]*downstreamdomain.OrderRef{ref.ID: ref}}
	orders := orderReaderStub{orders: map[uint]*downstreamcontract.OrderSnapshot{8: {ID: 8, Status: "paid"}}}
	credentials := credentialReaderStub{credentials: map[uint]*downstreamcontract.Credential{5: {ID: 5}}}
	queue := &callbackQueueStub{}
	service := newService(references, orders, credentials, queue, &delivererStub{err: errors.New("network unavailable")})

	if err := service.SendCallback(context.Background(), ref.ID); err != nil {
		t.Fatalf("SendCallback() should persist retry state, got %v", err)
	}
	if ref.CallbackRetryCount != 1 || len(queue.callbacks) != 1 || queue.callbacks[0].delay != 30*time.Second {
		t.Fatalf("retry state mismatch: ref=%#v queue=%#v", ref, queue.callbacks)
	}
}

func TestSendCallbackReturnsStableNotFoundSentinel(t *testing.T) {
	service := newService(&refRepositoryStub{}, orderReaderStub{}, credentialReaderStub{}, nil, &delivererStub{})
	if err := service.SendCallback(context.Background(), 404); !errors.Is(err, downstreamcontract.ErrRefNotFound) {
		t.Fatalf("error = %v, want ErrRefNotFound", err)
	}
}

type fixedEffectOwner struct{}

func (fixedEffectOwner) UsesFixedPaymentEffects(uint) (bool, error) { return true, nil }
func TestDurableCallbackPropagatesFailureWithoutRedisRetry(t *testing.T) {
	ref := &downstreamdomain.OrderRef{ID: 1, OrderID: 2, ApiCredentialID: 3, CallbackURL: "https://synthetic.example/callback"}
	repo := &refRepositoryStub{byID: map[uint]*downstreamdomain.OrderRef{1: ref}, byOrderID: map[uint]*downstreamdomain.OrderRef{2: ref}}
	queue := &callbackQueueStub{}
	deliverer := &durableDelivererStub{err: errors.New("synthetic failure")}
	order := &downstreamcontract.OrderSnapshot{ID: 2, OrderNo: "synthetic", Status: "paid"}
	svc := downstreamapp.NewService(downstreamapp.Options{References: repo, Orders: orderReaderStub{orders: map[uint]*downstreamcontract.OrderSnapshot{2: order}}, Credentials: credentialReaderStub{credentials: map[uint]*downstreamcontract.Credential{3: {APIKey: "test", APISecret: "test"}}}, Queue: queue, Deliverer: deliverer, PaymentEffects: fixedEffectOwner{}})
	if err := svc.SendCallback(context.Background(), 1); err != nil || deliverer.calls != 0 {
		t.Fatal("old queue bypassed fixed owner")
	}
	if err := svc.EnqueueCallbackReliable(2); err != nil || len(queue.callbacks) != 0 {
		t.Fatal("legacy task created for fixed payment")
	}
	permit := func(context.Context) error { return nil }
	if err := svc.SendCallbackDurable(context.Background(), 2, permit); err == nil || len(queue.callbacks) != 0 {
		t.Fatal("HTTP failure swallowed or independently requeued")
	}
	deliverer.err = nil
	if err := svc.SendCallbackDurable(context.Background(), 2, permit); err != nil {
		t.Fatal(err)
	}
	if deliverer.calls != 2 || ref.CallbackStatus != downstreamdomain.StatusSent {
		t.Fatal("callback not acknowledged")
	}
	frozen := errors.New("frozen")
	if err := svc.SendCallbackDurable(context.Background(), 2, func(context.Context) error { return frozen }); !errors.Is(err, frozen) || deliverer.calls != 2 {
		t.Fatal("frozen callback sent")
	}
	order.Status = "refunded"
	if err := svc.EnqueueCallbackReliable(2); err != nil || len(queue.callbacks) != 1 {
		t.Fatal("refund callback suppressed")
	}
	if err := svc.SendCallback(context.Background(), 1); err != nil || deliverer.calls != 3 {
		t.Fatal("refund callback not delivered")
	}
}

type durableDelivererStub struct {
	calls int
	err   error
}

func (d *durableDelivererStub) Send(context.Context, downstreamcontract.DeliveryRequest) error {
	d.calls++
	return d.err
}
