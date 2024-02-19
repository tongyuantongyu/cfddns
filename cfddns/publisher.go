package cfddns

import (
	"cfddns/config"
	"cfddns/ddns"
	"cfddns/log"
	"context"
	"fmt"
	"go.uber.org/zap"
	"net"
)

var DefaultMark = "cfddns"

type recordPublisher struct {
	name     string
	provider ddns.Interface
	record   ddns.Record
}

func (r *recordPublisher) init(ctx context.Context, config config.Domain) error {
	r.name = config.Address
	ctx = log.SWith(ctx, "name", r.name)

	r.record.Domain = config.Domain
	r.record.Type = config.Type
	r.record.Mark = DefaultMark
	if config.Mark != nil {
		r.record.Mark += "-" + *config.Mark
	}

	records, err := r.provider.FindRecord(ctx, r.record)
	if err != nil {
		log.S(ctx).Errorw("failed read record info", zap.Error(err))
		return err
	}

	if len(records) > 1 {
		log.S(ctx).Errorw("inconsistent state: found multiple records", "count", len(records))
		return fmt.Errorf("inconsistent state: found multiple records")
	}

	if len(records) != 0 {
		r.record = records[0]
		log.S(ctx).Infow("found record", "ip", r.record.Address)
	} else {
		log.S(ctx).Infow("no record found")
	}

	return nil
}

func (r *recordPublisher) update(ctx context.Context, ip net.IP) error {
	if r.record.Address == ip.String() {
		log.S(ctx).Infow("IP didn't change, skip update", "ip", ip, "domain", r.record.Domain, "ns_type", r.record.Type)
		return nil
	}

	oldIP := r.record.Address
	r.record.Address = ip.String()

	record, err := r.provider.WriteRecord(ctx, r.record)
	if err != nil {
		return fmt.Errorf("failed update domain: %w", err)
	}

	log.S(ctx).Infow("record updated", "ip", ip, "old_ip", oldIP, "domain", r.record.Domain, "ns_type", r.record.Type)
	r.record = record
	return nil
}

type Publisher struct {
	domains []*recordPublisher
}

func (p *Publisher) Publish(ctx context.Context, state map[string]net.IP) error {
	ctx = log.SWith(ctx, log.Stage("update"))
	for _, domain := range p.domains {
		ip := state[domain.name]
		if ip == nil {
			log.S(ctx).Warnw("ip not resolved, cannot update domain", "name", domain.name)
			continue
		}

		// Intentionally ignored. Partial success is better than all fail.
		_ = domain.update(ctx, ip)
	}

	return nil
}

func NewPublisher(ctx context.Context, pc config.CloudflareConfig, dc []config.Domain) (*Publisher, error) {
	ctx = log.SWith(ctx, log.Stage("init:publisher"))
	p := &Publisher{}

	pro, err := ddns.Providers["cloudflare"](ctx, pc)
	if err != nil {
		log.S(ctx).Errorw("failed loading provider", "provider", "cloudflare", zap.Error(err))
		return nil, fmt.Errorf("failed loading provider: %w", err)
	}

	for _, domain := range dc {
		rp := &recordPublisher{provider: pro}

		if err := rp.init(ctx, domain); err != nil {
			log.S(ctx).Errorw("failed init domain", "domain", domain.Domain, "ns_type", domain.Type, zap.Error(err))
			return nil, err
		}

		p.domains = append(p.domains, rp)
	}

	return p, nil
}
