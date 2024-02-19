package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type elapsed struct {
	t   time.Time
	key string
}

func (v *elapsed) MarshalLogObject(e zapcore.ObjectEncoder) error {
	e.AddDuration(v.key, time.Since(v.t))
	return nil
}

func Elapsed(key string) zap.Field {
	return zap.Inline(&elapsed{
		t:   time.Now(),
		key: key,
	})
}
