package billing

import (
	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/billing"
	"github.com/android-sms-gateway/server/internal/worker/executor"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"billing",
		logger.WithNamedLogger("billing"),
		fx.Provide(billing.NewRepository, fx.Private),
		fx.Provide(
			executor.AsWorkerTask(NewExpireVisibilityTask),
		),
	)
}
