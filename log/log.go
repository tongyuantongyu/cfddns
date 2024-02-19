package log

import (
	"context"

	"go.uber.org/zap"
)

type logCtx struct {
	context.Context

	logger  *zap.Logger
	sLogger *zap.SugaredLogger
}

type logType struct{}

func (c *logCtx) Value(k any) any {
	if _, ok := k.(logType); ok {
		return c.logger
	}

	return c.Context.Value(k)
}

func WithLogger(parent context.Context, logger *zap.Logger) context.Context {
	return &logCtx{Context: parent, logger: logger, sLogger: logger.Sugar()}
}

// L returns logger in context, or DefaultLogger if no logger is present
func L(ctx context.Context) *zap.Logger {
	if l, ok := ctx.(*logCtx); ok {
		return l.logger
	}

	if l, _ := ctx.Value(logType{}).(*zap.Logger); l != nil {
		return l
	}

	return zap.L()
}

// S returns sugared version of L.
func S(ctx context.Context) *zap.SugaredLogger {
	if s, ok := ctx.(*logCtx); ok {
		return s.sLogger
	}

	if s, _ := ctx.Value(logType{}).(*zap.Logger); s != nil {
		return s.Sugar()
	}

	return zap.S()
}

func With(ctx context.Context, tags ...zap.Field) context.Context {
	l := &logCtx{
		Context: ctx,
		logger:  L(ctx).With(tags...),
	}
	l.sLogger = l.logger.Sugar()
	return l
}

func SWith(ctx context.Context, tags ...interface{}) context.Context {
	l := &logCtx{
		Context: ctx,
		sLogger: S(ctx).With(tags...),
	}
	l.logger = l.sLogger.Desugar()
	return l
}
