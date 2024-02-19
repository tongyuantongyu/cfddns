package cfddns

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"cfddns/sources"
	"cfddns/transformers"
	"context"
	"fmt"
	"go.uber.org/zap"
	"net"
)

type ipResolver struct {
	sources      []sources.Interface
	transformers []transformers.Interface
}

func (r *ipResolver) resolve(ctx context.Context) (ip net.IP, err error) {
	sourceType := ""
Next:
	for _, source := range r.sources {
		ip, err = source.Lookup(ctx)
		if err != nil {
			continue
		}

		for _, transformer := range r.transformers {
			ip, err = transformer.Transform(ctx, ip)
			if err != nil {
				continue Next
			}
		}

		sourceType = source.Typename()
		break
	}

	if ip == nil {
		log.S(ctx).Errorw("all source failed, unable to get ip")
		return nil, fmt.Errorf("all source failed")
	}

	log.S(ctx).Infow("resolved ip", "ip", ip, "source_type", sourceType)

	return
}

type Resolver struct {
	list map[string]ipResolver
}

func (r Resolver) resolveOne(ctx context.Context, name string, table map[string]net.IP, left map[string]struct{}) (ip net.IP, err error) {
	ctx = log.SWith(ctx, "name", name)

	if ip_, exist := table[name]; exist {
		log.S(ctx).Debugw("found result in resolved table")
		return ip_, nil
	}

	res, exist := r.list[name]
	if !exist {
		log.S(ctx).Errorw("non-exist IP address entry")
		return nil, fmt.Errorf("non-exist IP address entry")
	}

	ip, err = res.resolve(ctx)
	delete(left, name)
	table[name] = ip

	return
}

func (r Resolver) Resolve(ctx context.Context) (result map[string]net.IP, err error) {
	ctx = log.SWith(ctx, log.Stage("resolve"))

	result = map[string]net.IP{}
	left := map[string]struct{}{}
	for addr := range r.list {
		left[addr] = struct{}{}
	}

	ctx = context.WithValue(ctx, common.SourceResolverKey, func(ctx context.Context, name string) (net.IP, error) {
		return r.resolveOne(ctx, name, result, left)
	})

	for len(left) > 0 {
		var name string
		for n := range left {
			name = n
			break
		}

		_, err = r.resolveOne(ctx, name, result, left)
		if err != nil {
			log.S(ctx).Errorw("resolve failed", "name", name, zap.Error(err))
			return nil, err
		}
	}

	return
}

func NewResolver(ctx context.Context, c []config.IPAddress) (*Resolver, error) {
	r := &Resolver{list: map[string]ipResolver{}}

	for _, addr := range c {
		res := ipResolver{}

		for _, s := range addr.Sources {
			ctx := log.SWith(ctx, log.Stage("init:source"), "name", addr.Name, "type", s.Type)
			create, ok := sources.Sources[s.Type]
			if !ok {
				log.S(ctx).Errorw("unknown source type")
				return nil, fmt.Errorf("unknown source type")
			}

			if source, err := create(ctx, s); err != nil {
				return nil, fmt.Errorf("failed creating source: %w", err)
			} else {
				res.sources = append(res.sources, source)
			}
		}

		for _, s := range addr.Transformers {
			ctx := log.SWith(ctx, log.Stage("init:transformer"), "name", addr.Name, "type", s.Type)
			create, ok := transformers.Transformers[s.Type]
			if !ok {
				log.S(ctx).Errorw("unknown transformer type")
				return nil, fmt.Errorf("unknown transformer type")
			}

			if transformer, err := create(ctx, s); err != nil {
				return nil, fmt.Errorf("failed creating transformer: %w", err)
			} else {
				res.transformers = append(res.transformers, transformer)
			}
		}

		r.list[addr.Name] = res
	}

	return r, nil
}
