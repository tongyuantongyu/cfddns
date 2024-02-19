package sources

import (
	"cfddns/log"
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
)

type transportDialer func(ctx context.Context, network, addr string) (net.Conn, error)

func wrapClientDialer(ctx context.Context, client *http.Client, wrapperBuilder func(upstream transportDialer) transportDialer) (*http.Client, error) {
	if client == nil {
		client = http.DefaultClient
	}

	transport := http.DefaultTransport.(*http.Transport)
	if client.Transport != nil {
		t, ok := client.Transport.(*http.Transport)
		if !ok {
			log.S(ctx).Errorw("found unknown custom http.Client.Transpose",
				"transpose_type", reflect.TypeOf(client.Transport).String())
			return nil, fmt.Errorf("unknown custom http.Client.Transpose")
		}

		transport = t
	}

	transport = transport.Clone()
	transport.DialContext = wrapperBuilder(transport.DialContext)

	if transport.DialTLSContext != nil {
		transport.DialTLSContext = wrapperBuilder(transport.DialTLSContext)
	}

	clientCopy := *client
	clientCopy.Transport = transport
	return &clientCopy, nil
}
