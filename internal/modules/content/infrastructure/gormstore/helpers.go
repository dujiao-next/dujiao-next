package gormstore

import (
	"context"

	"gorm.io/gorm"
)

func withContext(db *gorm.DB, ctx context.Context) *gorm.DB {
	if ctx == nil {
		return db
	}
	return db.WithContext(ctx)
}
