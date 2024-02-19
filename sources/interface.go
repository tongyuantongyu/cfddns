package sources

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"cfddns/sources/netif"
	"context"
	"fmt"
	"net"
	"slices"

	"go.uber.org/zap"
)

type networkInterface struct {
	config.IPSourceInterfaceConfig `mapstructure:",squash"`

	iface string
	flag  common.IPFilterFlag
}

func (s *networkInterface) Typename() string {
	return "interface"
}

func (s *networkInterface) Lookup(ctx context.Context) (result net.IP, err error) {
	ctx = log.SWith(ctx,
		"interface", s.iface,
		"family", s.Type,
		"select", s.Select,
		"flag", s.flag,
		zap.Stringers("exclude", s.Exclude),
		zap.Stringers("include", s.Include),
	)

	defer func() {
		if err == nil {
			log.S(ctx).Debugw("got ip", log.IP(result))
		}
	}()

	iface, err := netif.InterfaceByName(s.iface)
	if err != nil {
		log.S(ctx).Warnw("find interface failed", zap.Error(err))
		return nil, fmt.Errorf(`find interface failed: %w`, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		log.S(ctx).Warnw("get address failed", zap.Error(err))
		return nil, fmt.Errorf(`get address failed: %w`, err)
	}

	var candidate []net.IP

Next:
	for _, addr := range addrs {
		var ip net.IP
		flags := addr.Flags

		switch addr := addr.Addr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}

		ctx := log.SWith(ctx, log.IP(ip), "flag", flags, "raw_flags", addr.RawFlags)

		if (s.Type == common.IPv4) != (ip.To4() != nil) {
			log.S(ctx).Debugw("discard IP", "reason", "family mismatch")
			continue
		}

		if !s.flag.Match(common.FlagNonGlobalUnicast) && !ip.IsGlobalUnicast() {
			log.S(ctx).Debugw("discard IP", "reason", "ignore non Global Unicast IP")
			continue
		}

		if !s.flag.Match(common.FlagPrivate) && ip.IsPrivate() {
			log.S(ctx).Debugw("discard IP", "reason", "ignore Private IP")
			continue
		}

		if s.flag.Match(common.FlagNoEUI64) && len(ip) == net.IPv6len {
			if ip[11] == 0xff && ip[12] == 0xfe {
				log.S(ctx).Debugw("discard IP", "reason", "ignore EUI64 IP")
				continue
			}
		}

		if !s.flag.Match(common.FlagTemporary) && flags&netif.FlagTemporary != 0 {
			log.S(ctx).Debugw("discard IP", "reason", "ignore temporary IP")
			continue
		}

		if !s.flag.Match(common.FlagBadDad) && flags&(netif.FlagDadDuplicated|netif.FlagDadTentative) != 0 {
			log.S(ctx).Debugw("discard IP", "reason", "ignore IP with bad DAD state")
			continue
		}

		if !s.flag.Match(common.FlagDeprecated) && flags&netif.FlagDeprecated != 0 {
			log.S(ctx).Debugw("discard IP", "reason", "ignore deprecated IP")
			continue
		}

		for _, ex := range s.Exclude {
			if ex.Contains(ip) {
				log.S(ctx).Debugw("discard IP", "reason", "in exclude CIDR", "cidr", ex)
				continue Next
			}
		}

		if s.Include != nil {
			matched := false
			for _, ic := range s.Include {
				if ic.Contains(ip) {
					matched = true
					break
				}
			}

			if !matched {
				log.S(ctx).Debugw("discard IP", "reason", "not in any include CIDR")
				continue
			}
		}

		log.S(ctx).Debugw("add IP to candidate")
		candidate = append(candidate, ip)
	}

	if len(candidate) == 0 {
		log.S(ctx).Warnw("no eligible IP found")
		return nil, fmt.Errorf(`no eligible IP found`)
	}

	switch s.Select {
	case common.SelectShortest:
		slices.SortStableFunc(candidate, func(i, j net.IP) int {
			return len(i.String()) - len(j.String())
		})
		fallthrough
	case common.SelectFirst:
		return candidate[0], nil
	case common.SelectLast:
		return candidate[len(candidate)-1], nil
	default:
		log.S(ctx).Errorw("unexpected select mode")
		return nil, fmt.Errorf(`internal error: unexpected select mode`)
	}
}

func newInterface(ctx context.Context, config config.IPSource) (Interface, error) {
	ctx = log.SWith(ctx, "type", "interface")

	s := &networkInterface{iface: config.Source}
	if err := common.WeakDecodeMap(config.Config, s); err != nil {
		log.S(ctx).Errorw("bad config", zap.Error(err), "config", config.Config)
		return nil, fmt.Errorf(`bad config: %w`, err)
	}

	for _, f := range s.Flags {
		s.flag |= f
	}

	return s, nil
}
