package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPaymentNotFound = errors.New("payment not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	var sub Subscription
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil //nolint:nilnil // "no subscription yet" is a valid, common state
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return &sub, nil
}

// ExtendSubscription sets current_period_end to one month past the later of "now"
// and the existing period end (so renewing early doesn't lose unused time), creating
// the row if it doesn't exist yet.
func (r *Repository) ExtendSubscription(ctx context.Context, userID string) error {
	now := time.Now()

	existing, err := r.GetSubscription(ctx, userID)
	if err != nil {
		return err
	}

	base := now
	if existing != nil && existing.CurrentPeriodEnd.After(base) {
		base = existing.CurrentPeriodEnd
	}
	newPeriodEnd := base.AddDate(0, 1, 0)

	sub := Subscription{UserID: userID, CurrentPeriodEnd: newPeriodEnd}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"current_period_end"}),
		}).
		Create(&sub).Error; err != nil {
		return fmt.Errorf("failed to extend subscription: %w", err)
	}

	return nil
}

func (r *Repository) InsertPayment(ctx context.Context, payment *Payment) error {
	if err := r.db.WithContext(ctx).Create(payment).Error; err != nil {
		return fmt.Errorf("failed to insert payment: %w", err)
	}
	return nil
}

func (r *Repository) GetPaymentByReference(ctx context.Context, referenceNumber string) (*Payment, error) {
	var payment Payment
	if err := r.db.WithContext(ctx).Where("pesepay_reference_number = ?", referenceNumber).First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

func (r *Repository) MarkPaymentStatus(ctx context.Context, referenceNumber string, status paymentStatus) error {
	if err := r.db.WithContext(ctx).Model(new(Payment)).
		Where("pesepay_reference_number = ?", referenceNumber).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}
	return nil
}

// ListRecentlyExpired is used only by the worker's visibility task, not by any
// access-control decision (IsActive is purely date-based).
func (r *Repository) ListRecentlyExpired(ctx context.Context, since time.Time) ([]Subscription, error) {
	var subs []Subscription
	if err := r.db.WithContext(ctx).
		Where("current_period_end >= ? AND current_period_end < ?", since, time.Now()).
		Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("failed to list recently expired subscriptions: %w", err)
	}
	return subs, nil
}
