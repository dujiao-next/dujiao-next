package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动（基于 modernc.org/sqlite）
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DBPoolConfig 数据库连接池配置
type DBPoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

// InitDB 初始化数据库连接
func InitDB(driver, dsn string, pool DBPoolConfig) error {
	var err error
	normalized := strings.ToLower(strings.TrimSpace(driver))
	var dialector gorm.Dialector
	switch normalized {
	case "", "sqlite":
		// glebarez/sqlite 是基于 modernc.org/sqlite 的纯 Go 驱动
		dialector = sqlite.Open(dsn)
	case "mysql", "mariadb":
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	applyDBPool(sqlDB, pool)
	return nil
}

func applyDBPool(sqlDB *sql.DB, pool DBPoolConfig) {
	if sqlDB == nil {
		return
	}
	if pool.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns >= 0 {
		sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetimeSeconds) * time.Second)
	}
	if pool.ConnMaxIdleTimeSeconds > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTimeSeconds) * time.Second)
	}
}

// AutoMigrate 自动迁移所有数据库表
func AutoMigrate() error {
	return DB.AutoMigrate(
		&Admin{},
		&User{},
		&UserLoginLog{},
		&AuthzAuditLog{},
		&EmailVerifyCode{},
		&Order{},
		&OrderItem{},
		&CartItem{},
		&PaymentChannel{},
		&Payment{},
		&CardSecret{},
		&CardSecretBatch{},
		&Fulfillment{},
		&Coupon{},
		&CouponUsage{},
		&Promotion{},
		&Category{},
		&Product{},
		&Post{},
		&Banner{},
		&Setting{},
	)
}
