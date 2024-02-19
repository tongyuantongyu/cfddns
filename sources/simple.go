package sources

import (
	"cfddns/common"
	"cfddns/config"
	"cfddns/log"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"time"

	"go.uber.org/zap"
)

const maxReadSimple = 4 * 1024

var ipRegex = []*regexp.Regexp{
	regexp.MustCompile("([0-9]{1,3}\\\\.){3}[0-9]{1,3}(\\\\/([0-9]|[1-2][0-9]|3[0-2]))?"),
	regexp.MustCompile("s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\\/([0-9]|[1-9][0-9]|1[0-1][0-9]|12[0-8]))?"),
}

type simple struct {
	config.IPSourceSimpleConfig `mapstructure:",squash"`

	url string
}

func (s *simple) Typename() string {
	return "simple"
}

func (s *simple) wrapDialer(upstream transportDialer) transportDialer {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		switch s.Type {
		case common.IPv4:
			network += "4"
		case common.IPv6:
			network += "6"
		}

		return upstream(ctx, network, addr)
	}
}

func (s *simple) Lookup(ctx context.Context) (result net.IP, err error) {
	client := http.DefaultClient
	timeout := time.Duration(s.Timeout)

	if ctxClient := ctx.Value(common.HttpClientKey); ctxClient != nil {
		client = ctxClient.(*http.Client)
	}

	log.S(ctx).Debug("patching http.Client")

	client, err = wrapClientDialer(ctx, client, s.wrapDialer)
	if err != nil {
		return nil, err
	}

	ctx = log.SWith(ctx, "url", s.url, "family", s.Type, "timeout", timeout)

	defer func() {
		if err == nil {
			log.S(ctx).Debugw("got ip", log.IP(result))
		}
	}()

	if s.Timeout > 0 {
		tCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = tCtx
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		log.S(ctx).Errorw("new request failed", zap.Error(err))
		return nil, fmt.Errorf("new request failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.S(ctx).Warnw("connection failed", zap.Error(err))
		return nil, fmt.Errorf(`connection failed: %w`, err)
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.S(ctx).Warnw("close body failed", zap.Error(err))
		}
	}(resp.Body)

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReadSimple))
	if err != nil {
		log.S(ctx).Warnw("receiving response failed", zap.Error(err))
		return nil, fmt.Errorf(`failed receiving response: %w`, err)
	}

	ipData := ipRegex[s.Type].Find(data)
	if ipData == nil {
		log.S(ctx).Warnw("no IP found in response", log.ByteField("body", data))
		return nil, fmt.Errorf("no IP found in response")
	}

	ipString := string(ipData)
	nip, err := netip.ParseAddr(ipString)
	if err != nil {
		log.S(ctx).Errorw("found bad IP", "ip", ipString, zap.Error(err), log.Internal)
		return nil, fmt.Errorf(`internal error: found bad IP`)
	}

	switch {
	case nip.Zone() != "":
		log.S(ctx).Warnw("found zone in IP", "ip", ipString, "zone", nip.Zone())
		return nil, fmt.Errorf(`unsupported: found zone in IP`)

	case (nip.Is4() || nip.Is4In6()) && s.Type == common.IPv4:
		ip := nip.As4()
		return ip[:], nil

	case nip.Is6() && s.Type == common.IPv6:
		ip := nip.As16()
		return ip[:], nil
	default:
		log.S(ctx).Errorw("mismatched IP family", "ip", ipString, log.Internal)
		return nil, fmt.Errorf(`internal error: mismatched IP family`)
	}
}

func newSimple(ctx context.Context, config config.IPSource) (Interface, error) {
	ctx = log.SWith(ctx, "type", "simple")

	s := &simple{url: config.Source}
	if err := common.WeakDecodeMap(config.Config, s); err != nil {
		log.S(ctx).Errorw("bad config", zap.Error(err), "config", config.Config)
		return nil, fmt.Errorf(`bad config: %w`, err)
	}

	return s, nil
}
