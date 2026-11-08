package billing

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"billing",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("billing")
		}),
		fx.Provide(NewRepository),
		fx.Provide(NewService),
	)
}
