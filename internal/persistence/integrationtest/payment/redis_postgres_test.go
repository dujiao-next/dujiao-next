package settlement_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	cs "github.com/dujiao-next/internal/modules/cardsecret/domain"
	fa "github.com/dujiao-next/internal/modules/fulfillment/application"
	fd "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fs "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	osstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	app "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/settlement"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	qa "github.com/dujiao-next/internal/modules/payment/infrastructure/queueadapter"
	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"
)

func TestPostgresPaymentRecoversLostRedisQueueAndDeliversOnce(t *testing.T) {
	if os.Getenv("PAYMENT_TEST_REDIS_HOST") != "dujiao-payment-redis13" {
		t.Skip("isolated Redis host not selected")
	}
	db := paymentPostgres(t)
	ctx := context.Background()
	opts := asynq.RedisClientOpt{Addr: "dujiao-payment-redis13:6379", DB: 15}
	inspect := asynq.NewInspector(opts)
	defer inspect.Close()
	// This integration DB is dedicated to the disposable lab. Refuse any old queue.
	if info, err := inspect.GetQueueInfo(queue.DefaultQueue); err == nil && info.Size != 0 {
		t.Fatal("test queue is not empty")
	}
	qclient, err := queue.NewClient(&config.QueueConfig{Enabled: true, Host: "dujiao-payment-redis13", Port: 6379, DB: 15})
	must(t, err)
	defer qclient.Close()
	q := qa.New(qclient)
	p, _ := seed(t, db, 1, 1)
	evidence(t, db, p, "1", "redis-recovery-hash")
	svc, _, repo := services(db, &verifier{})
	must(t, svc.Run(ctx))
	must(t, db.Create(&cs.Secret{ProductID: 1, SKUID: 1, Secret: "synthetic-only-card", Status: cs.StatusAvailable}).Error)
	must(t, db.Model(&pd.PaymentEffect{}).Where("kind <> ?", "auto_fulfill").Update("state", "done").Error)
	orders := osstore.New(db, "test-guest-key")
	application := app.NewPaymentService(app.PaymentServiceOptions{OrderStore: orders, PaymentStore: repo, Queue: q})
	runner := &settlement.Effects{Store: repo, Handler: application}
	must(t, runner.Run(ctx))
	info, err := inspect.GetQueueInfo(queue.DefaultQueue)
	must(t, err)
	if info.Pending != 1 {
		t.Fatal("SQL outbox did not enqueue real Redis task")
	}
	deleted, err := inspect.DeleteAllPendingTasks(queue.DefaultQueue)
	must(t, err)
	if deleted != 1 {
		t.Fatal("failed to simulate lost Redis job")
	}
	var work pd.PaymentEffect
	must(t, db.Where("kind = ?", "auto_fulfill").First(&work).Error)
	if work.State != "pending" || count(t, db, &fd.Fulfillment{}) != 0 {
		t.Fatal("queue ACK was confused with fulfillment")
	}
	must(t, db.Model(&work).Update("after", time.Now().Add(-time.Second)).Error)
	must(t, runner.Run(ctx))
	// Duplicate the queued job too; the real registered consumer must not deliver twice.
	must(t, q.EnqueueVerifiedOrderAutoFulfill(p.OrderID))
	delivery := fa.New(fa.Options{OrderStore: orders, FulfillmentStore: fs.New(db)})
	mux := asynq.NewServeMux()
	consumer.New(&container.Container{OrderStore: orders, FulfillmentService: delivery}).Register(mux)
	worker := asynq.NewServer(opts, asynq.Config{Concurrency: 2, Queues: map[string]int{queue.DefaultQueue: 1}, ShutdownTimeout: time.Second, TaskCheckInterval: 100 * time.Millisecond})
	must(t, worker.Start(mux))
	defer worker.Shutdown()
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err = inspect.GetQueueInfo(queue.DefaultQueue)
		must(t, err)
		if info.Pending == 0 && info.Active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("real Redis consumer did not drain")
		}
		time.Sleep(50 * time.Millisecond)
	}
	must(t, db.Model(&pd.PaymentEffect{}).Where("id = ?", work.ID).Update("after", time.Now().Add(-time.Second)).Error)
	must(t, runner.Run(ctx))
	must(t, db.First(&work, work.ID).Error)
	var used int64
	must(t, db.Model(&cs.Secret{}).Where("status = ?", cs.StatusUsed).Count(&used).Error)
	if work.State != "done" || count(t, db, &fd.Fulfillment{}) != 1 || used != 1 {
		t.Fatal("queue recovery lost or duplicated delivery")
	}
}
