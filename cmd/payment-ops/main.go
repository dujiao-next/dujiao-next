// payment-ops performs local operator actions without HTTP, workers or migrations.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dujiao-next/internal/config"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"os"
	"time"
)

func main() {
	action := flag.String("action", "status", "status, freeze, resume, note")
	revision := flag.Uint64("revision", 0, "revision observed immediately before change")
	operator := flag.String("operator", "", "operator identity for the audit trail")
	reason := flag.String("reason", "", "reason, up to 160 bytes")
	paymentID := flag.Uint("payment", 0, "fixed payment ID for an audit note")
	flag.Parse()
	cfg := config.Load()
	if err := gormdb.InitDB(cfg.Database.Driver, cfg.Database.DSN, gormdb.DBPoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}, cfg.Server.Mode); err != nil {
		fail("database connection failed")
	}
	raw, err := gormdb.DB.DB()
	if err != nil {
		fail("database unavailable")
	}
	defer raw.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if *action != "status" {
		if err := automation.Operate(ctx, gormdb.DB, *action, *operator, *reason, *revision, *paymentID); err != nil {
			fail("operation failed; inspect current revision and schema, no success is implied")
		}
	}
	var control pd.AutomationControl
	if gormdb.DB.WithContext(ctx).First(&control, 1).Error != nil {
		fail("control missing or unavailable")
	}
	if json.NewEncoder(os.Stdout).Encode(control) != nil {
		fail("output failed")
	}
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
