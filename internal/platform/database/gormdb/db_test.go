package gormdb

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type gormLogRecorder struct {
	bytes.Buffer
}

func (recorder *gormLogRecorder) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(&recorder.Buffer, format, args...)
}

func TestReleaseGORMLoggerDoesNotExposeQueryParameters(t *testing.T) {
	recorder := &gormLogRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: newGORMLogger(" release ", recorder),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	secret := "must-not-appear-in-production-database-logs"
	if err := db.Exec("INSERT INTO missing_table (secret) VALUES (?)", secret).Error; err == nil {
		t.Fatal("expected missing-table error")
	}

	output := recorder.String()
	if output == "" {
		t.Fatal("expected database error to be logged")
	}
	if strings.Contains(output, secret) {
		t.Fatalf("release database log exposed a query parameter: %s", output)
	}
	if !strings.Contains(output, "missing_table") {
		t.Fatalf("release database log should retain useful query context: %s", output)
	}
}

func TestSQLiteReadPoolDoesNotBlockOnOpenWriteTransaction(t *testing.T) {
	previous := DB
	t.Cleanup(func() { DB = previous })

	dsn := filepath.Join(t.TempDir(), "readpool.db")
	if err := InitDB("sqlite", dsn, DBPoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}, "release"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if err := DB.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := DB.Exec("INSERT INTO items (id) VALUES (1)").Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	count := func() int64 {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var n int64
		if err := DB.WithContext(ctx).Table("items").Count(&n).Error; err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	// 事务占住唯一的写连接；读必须走读池立即返回，且看不到未提交的行
	tx := DB.Begin()
	if err := tx.Exec("INSERT INTO items (id) VALUES (2)").Error; err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if got := count(); got != 1 {
		t.Fatalf("read during open tx = %d, want 1", got)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := count(); got != 2 {
		t.Fatalf("read after commit = %d, want 2", got)
	}

	if err := DB.Clauses(dbresolver.Read).Exec("INSERT INTO items (id) VALUES (3)").Error; err == nil {
		t.Fatal("write through read pool succeeded, want query_only error")
	}
}
