package transformers

import (
	"cfddns/config"
	"context"
	"net"
)

type Interface interface {
	Transform(ctx context.Context, ip net.IP) (net.IP, error)
}

var Transformers = map[string]func(ctx context.Context, transformer config.IPTransformer) (Interface, error){
	"mask_rewrite": newMaskRewrite,
}
