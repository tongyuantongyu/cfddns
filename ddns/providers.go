package ddns

import (
	"cfddns/config"
	"context"
)

type Interface interface {
	FindRecord(ctx context.Context, r Record) ([]Record, error)
	WriteRecord(ctx context.Context, r Record) (Record, error)
}

type Record struct {
	Handle  any
	Domain  string
	Type    string
	Address string
	Mark    string
}

var Providers = map[string]func(ctx context.Context, provider config.CloudflareConfig) (Interface, error){
	"cloudflare": newCloudflare,
}
