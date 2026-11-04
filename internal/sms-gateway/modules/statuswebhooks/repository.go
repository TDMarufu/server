package statuswebhooks

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Select(filters ...SelectFilter) ([]*StatusWebhook, error) {
	webhooks := []*StatusWebhook{}
	if err := newFilter(filters...).apply(r.db).Find(&webhooks).Error; err != nil {
		return nil, fmt.Errorf("failed to select status webhooks: %w", err)
	}
	return webhooks, nil
}

// SelectActiveForEvent returns every non-disabled webhook across all users that
// fires on the given event - used by the dispatcher, which fans events out by user ID.
func (r *Repository) SelectActiveForEvent(userID, event string) ([]*StatusWebhook, error) {
	return r.Select(WithUserID(userID), withActiveForEvent(event))
}

func (r *Repository) Replace(webhook *StatusWebhook) error {
	if err := r.db.
		Clauses(clause.OnConflict{UpdateAll: true}).
		Save(webhook).Error; err != nil {
		return fmt.Errorf("failed to replace status webhook: %w", err)
	}
	return nil
}

func (r *Repository) Delete(filters ...SelectFilter) error {
	if err := newFilter(filters...).apply(r.db).Delete(new(StatusWebhook)).Error; err != nil {
		return fmt.Errorf("failed to delete status webhook: %w", err)
	}
	return nil
}

// RecordSuccess resets the failure counter after a successful delivery.
func (r *Repository) RecordSuccess(ctx context.Context, id uint64) error {
	if err := r.db.WithContext(ctx).Model(new(StatusWebhook)).
		Where("id = ?", id).
		Update("failure_count", 0).Error; err != nil {
		return fmt.Errorf("failed to record success: %w", err)
	}
	return nil
}

// RecordFailure increments the failure counter and disables the webhook once it
// reaches maxConsecutiveFailures, so a dead URL isn't retried forever.
func (r *Repository) RecordFailure(ctx context.Context, id uint64) error {
	if err := r.db.WithContext(ctx).Model(new(StatusWebhook)).
		Where("id = ?", id).
		Updates(map[string]any{
			"failure_count": gorm.Expr("failure_count + 1"),
			"disabled":      gorm.Expr("failure_count + 1 >= ?", maxConsecutiveFailures),
		}).Error; err != nil {
		return fmt.Errorf("failed to record failure: %w", err)
	}
	return nil
}
