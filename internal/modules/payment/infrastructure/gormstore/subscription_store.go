package gormstore

import (
	"context"
	"errors"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"gorm.io/gorm/clause"
	"time"
)

func (r *Store) SaveCandidateSubscription(ctx context.Context, source pd.CandidateSource, channelType string, cfg jsonmap.JSON) error {
	if source.GatewayURL == "" || source.PID == "" || len(cfg) == 0 {
		return errors.New("invalid subscription")
	}
	// Same identity may rotate credentials via the current trusted channel.
	// A new endpoint/PID appends a source; historical sources are never deleted.
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_key"}}, DoUpdates: clause.AssignmentColumns([]string{"config_json", "channel_type"})}).Create(&pd.CandidateSubscription{SourceKey: source.Key(), ChannelType: channelType, ConfigJSON: cfg, CreatedAt: time.Now()}).Error
}
func (r *Store) ListCandidateSubscriptions(ctx context.Context, page, size int) ([]pd.CandidateSubscription, error) {
	if page < 1 || size < 1 || size > 50 {
		return nil, errors.New("invalid subscription page")
	}
	var rows []pd.CandidateSubscription
	err := r.db.WithContext(ctx).Order("source_key ASC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, err
}
func (r *Store) RecordCandidatePoll(ctx context.Context, key string, success, healthy bool, now time.Time) error {
	updates := map[string]interface{}{"last_attempt_at": now, "scanner_healthy": healthy}
	if success {
		updates["last_success_at"] = now
	}
	return r.db.WithContext(ctx).Model(&pd.CandidateSubscription{}).Where("source_key = ?", key).Updates(updates).Error
}
