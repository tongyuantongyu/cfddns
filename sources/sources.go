package sources

import (
	"cfddns/config"
	"context"
	"net"
)

type Interface interface {
	Lookup(ctx context.Context) (net.IP, error)
	Typename() string
}

var Sources = map[string]func(ctx context.Context, source config.IPSource) (Interface, error){
	"simple":    newSimple,
	"cf_trace":  newCloudflareTrace,
	"interface": newInterface,
	"reference": newReference,
}
