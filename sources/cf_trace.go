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
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"
)

const maxReadCloudflareTrace = 1024
const defaultCloudflareDomain = "www.cloudflare.com"

type cloudflareTrace struct {
	config.IPSourceCloudflareTraceConfig `mapstructure:",squash"`

	host string
}

func (s *cloudflareTrace) Typename() string {
	return "cf-trace"
}

type transportDialer func(ctx context.Context, network, addr string) (net.Conn, error)

func (s *cloudflareTrace) wrapDialer(upstream transportDialer) transportDialer {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if s.ForceAddress != "" {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			addr = net.JoinHostPort(s.ForceAddress, port)
		}

		switch {
		case s.Type == nil:
			// pass
		case *s.Type == common.IPv4:
			network += "4"
		case *s.Type == common.IPv6:
			network += "6"
		}

		return upstream(ctx, network, addr)
	}
}

func (s *cloudflareTrace) Lookup(ctx context.Context) (result net.IP, err error) {
	client := http.DefaultClient
	timeout := time.Duration(s.Timeout)

	if ctxClient := ctx.Value(common.HttpClientKey); ctxClient != nil {
		client = ctxClient.(*http.Client)
	}

	ctx = log.SWith(ctx,
		"host", s.host,
		"family", s.Type,
		"force_addr", s.ForceAddress,
		"timeout", timeout)

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

	if s.ForceAddress != "" || s.Type != nil {
		log.S(ctx).Debug("patching http.Client")
		transport := http.DefaultTransport.(*http.Transport)
		if client.Transport != nil {
			t, ok := client.Transport.(*http.Transport)
			if !ok {
				log.S(ctx).Errorw("found unknown custom http.Client.Transpose",
					"transpose_type", reflect.TypeOf(client.Transport).String())
				return nil, fmt.Errorf("unknown custom http.Client.Transpose")
			}

			transport = t.Clone()
		}

		transport.DialContext = s.wrapDialer(transport.DialContext)

		if transport.DialTLSContext != nil {
			transport.DialTLSContext = s.wrapDialer(transport.DialTLSContext)
		}

		clientCopy := *client
		clientCopy.Transport = transport
		client = &clientCopy
	}

	url := fmt.Sprintf("https://%s/cdn-cgi/trace", s.host)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReadCloudflareTrace))
	if err != nil {
		log.S(ctx).Warnw("receiving response failed", zap.Error(err))
		return nil, fmt.Errorf(`failed receiving response: %w`, err)
	}

	ipString := ""
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ip=") {
			ipString = strings.TrimPrefix(line, "ip=")
			break
		}
	}

	if ipString == "" {
		log.S(ctx).Warnw("no IP found in response", log.ByteField("body", data))
		return nil, fmt.Errorf("no IP found in response")
	}

	nip, err := netip.ParseAddr(ipString)
	if err != nil {
		log.S(ctx).Errorw("found bad IP", "ip", ipString, zap.Error(err))
		return nil, fmt.Errorf(`found bad IP: %w`, err)
	}

	switch {
	case nip.Zone() != "":
		log.S(ctx).Warnw("found zone in IP", "ip", ipString, "zone", nip.Zone())
		return nil, fmt.Errorf(`unsupported: found zone in IP`)

	case s.Type == nil:
		fallthrough
	case nip.Is6() && *s.Type == common.IPv6:
		ip := nip.As16()
		return ip[:], nil

	case (nip.Is4() || nip.Is4In6()) && *s.Type == common.IPv4:
		ip := nip.As4()
		return ip[:], nil

	default:
		log.S(ctx).Errorw("mismatched IP family", "ip", ipString, log.Internal)
		return nil, fmt.Errorf(`internal error: mismatched IP family`)
	}
}

func newCloudflareTrace(ctx context.Context, config config.IPSource) (Interface, error) {
	ctx = log.SWith(ctx, "type", "cf_trace")

	host, isIP := common.DetectNormalizeAddr(config.Source)
	s := &cloudflareTrace{host: host}

	if err := common.WeakDecodeMap(config.Config, s); err != nil {
		log.S(ctx).Errorw("bad config", zap.Error(err), "config", config.Config)
		return nil, fmt.Errorf(`bad config: %w`, err)
	}

	if !s.IPHost && isIP {
		s.ForceAddress = s.host
		s.host = defaultCloudflareDomain
	}

	if strings.Contains(s.host, ":") {
		s.host = fmt.Sprintf("[%s]", s.host)
	}

	return s, nil
}
