package log

import "go.uber.org/zap"

var (
	// Internal mark the error severe, due to issues in code.
	Internal = zap.String("severe_error", "internal")
)
