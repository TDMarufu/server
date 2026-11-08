package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/billing"
	"github.com/android-sms-gateway/server/internal/worker/executor"
	"go.uber.org/zap"
)

// expireVisibilityTask is log-only, for ops visibility - it does NOT gate access
// itself. Access control (billing.Service.IsActive) is purely date-based on
// current_period_end, avoiding a separate status flag that could drift out of sync.
type expireVisibilityTask struct {
	config        Config
	subscriptions *billing.Repository

	logger *zap.Logger
}

func NewExpireVisibilityTask(
	config Config,
	subscriptions *billing.Repository,
	logger *zap.Logger,
) executor.PeriodicTask {
	return &expireVisibilityTask{
		config:        config,
		subscriptions: subscriptions,

		logger: logger,
	}
}

func (t *expireVisibilityTask) Interval() time.Duration {
	return t.config.Interval
}

func (t *expireVisibilityTask) Name() string {
	return "billing:expire-visibility"
}

func (t *expireVisibilityTask) Run(ctx context.Context) error {
	expired, err := t.subscriptions.ListRecentlyExpired(ctx, time.Now().Add(-t.config.Interval))
	if err != nil {
		return fmt.Errorf("failed to list recently expired subscriptions: %w", err)
	}

	for _, sub := range expired {
		t.logger.Info("subscription expired",
			zap.String("user_id", sub.UserID), zap.Time("current_period_end", sub.CurrentPeriodEnd))
	}

	return nil
}

var _ executor.PeriodicTask = (*expireVisibilityTask)(nil)
