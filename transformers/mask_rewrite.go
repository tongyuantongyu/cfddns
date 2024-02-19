package transformers

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"go.uber.org/zap"
)

type maskRewrite struct {
	mask      net.IPMask
	overwrite net.IP
}

func (t *maskRewrite) Transform(ctx context.Context, ip net.IP) (result net.IP, err error) {
	ctx = log.SWith(ctx, "overwrite", t.overwrite, "mask", t.mask)

	if len(ip) != len(t.overwrite) {
		log.S(ctx).Warnw("mismatched IP family", log.IP(ip))
		return nil, fmt.Errorf(`mismatched IP family`)
	}

	result = make(net.IP, len(ip))

	for i := 0; i < len(ip); i++ {
		result[i] = (ip[i] & t.mask[i]) | (t.overwrite[i] & ^t.mask[i])
	}

	log.S(ctx).Debugw("transformed ip", log.IP(result))

	return
}

func newMaskRewrite(ctx context.Context, conf config.IPTransformer) (Interface, error) {
	ctx = log.SWith(ctx, "type", "mask_rewrite")

	s := &maskRewrite{}

	var c config.IPTransformerMaskRewriteConfig

	if err := common.WeakDecodeMap(conf.Config, &c); err != nil {
		log.S(ctx).Errorw("bad conf", zap.Error(err), "conf", conf.Config)
		return nil, fmt.Errorf(`bad conf: %w`, err)
	}

	s.overwrite = c.Overwrite.IP

	if cidr, err := strconv.ParseUint(c.Mask, 10, 8); err == nil {
		bits := len(s.overwrite) * 8
		if cidr > uint64(bits) {
			log.S(ctx).Errorw("bad conf: CIDR out of range", "overwrite", s.overwrite, "cidr", cidr)
			return nil, fmt.Errorf("bad conf: CIDR out of range")
		}
		s.mask = net.CIDRMask(int(cidr), bits)
	} else {
		mask, err := netip.ParseAddr(c.Mask)
		if err != nil {
			log.S(ctx).Errorw("bad conf: bad mask", zap.Error(err), "mask", c.Mask)
			return nil, fmt.Errorf(`bad conf: bad mask: %w`, err)
		}

		s.mask = mask.AsSlice()
		if len(s.mask) != len(s.overwrite) {
			log.S(ctx).Errorw("mask and overwrite has mismatched IP family", "mask", s.mask, "overwrite", s.overwrite)
			return nil, fmt.Errorf(`bad conf: mismatch IP family`)
		}
	}

	return s, nil
}
