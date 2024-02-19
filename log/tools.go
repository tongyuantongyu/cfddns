package log

import (
	"net"
	"unicode/utf8"

	"go.uber.org/zap"
)

func ByteField(key string, data []byte) zap.Field {
	if utf8.Valid(data) {
		return zap.ByteString(key, data)
	} else {
		return zap.Binary(key, data)
	}
}

func IP(ip net.IP) zap.Field {
	return zap.Stringer("ip", ip)
}

func Stage(stage string) zap.Field {
	return zap.String("stage", stage)
}
