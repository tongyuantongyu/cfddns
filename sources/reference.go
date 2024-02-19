package sources

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"context"
	"fmt"
	"net"
)

type reference struct {
	name string
}

func (s *reference) Typename() string {
	return "reference"
}

type referenceRecursiveDetectorType struct{}

var referenceRecursiveDetectorKey referenceRecursiveDetectorType

func (s *reference) Lookup(ctx context.Context) (net.IP, error) {
	ctx = log.SWith(ctx, "upstream", s.name)

	refChainI := ctx.Value(referenceRecursiveDetectorKey)
	if refChainI != nil {
		refChain := refChainI.(*[]string)
		*refChain = append(*refChain, s.name)
		for _, node := range (*refChain)[:len(*refChain)-1] {
			if node == s.name {
				log.S(ctx).Errorw("infinite loop detected in IP source chain", "chain", *refChain)
				return nil, fmt.Errorf("infinite loop detected")
			}
		}
	} else {
		chain := []string{s.name}
		ctx = context.WithValue(ctx, referenceRecursiveDetectorKey, &chain)
	}

	resolverI := ctx.Value(common.SourceResolverKey)
	if resolverI == nil {
		log.S(ctx).Errorw("source resolver not found", log.Internal)
		return nil, fmt.Errorf("source resolver not found")
	}

	resolver := resolverI.(func(context.Context, string) (net.IP, error))
	return resolver(ctx, s.name)
}

func newReference(ctx context.Context, config config.IPSource) (Interface, error) {
	return &reference{name: config.Source}, nil
}
