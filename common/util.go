package common

import (
	"encoding"
	"net"
	"net/netip"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func WeakDecodeMap(input, output any) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   output,
		// WeaklyTypedInput: true,
		DecodeHook: func(
			f reflect.Type,
			t reflect.Type,
			data interface{}) (interface{}, error) {
			if !reflect.PointerTo(t).Implements(textUnmarshalerType) {
				return data, nil
			}

			str, ok := data.(string)
			if !ok {
				return data, nil
			}

			v := reflect.New(t).Interface().(encoding.TextUnmarshaler)
			if err := v.UnmarshalText([]byte(str)); err != nil {
				return nil, err
			}

			return v, nil
		},
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

func DetectNormalizeAddr(addr string) (norm string, isIP bool) {
	if _, err := netip.ParseAddr(addr); err == nil {
		return addr, true
	}

	if len(addr) > 2 && addr[0] == '[' && addr[len(addr)-1] == ']' {
		addrStrip := addr[1 : len(addr)-1]
		if ip, err := netip.ParseAddr(addrStrip); err == nil {
			if ip.Is6() {
				return addrStrip, true
			}
		}
	}

	return addr, false
}

type IP struct {
	net.IP
}

func (i *IP) UnmarshalText(b []byte) error {
	ip, err := netip.ParseAddr(string(b))
	if err != nil {
		return err
	}

	i.IP = ip.AsSlice()
	return nil
}

type CIDR struct {
	*net.IPNet
}

func (c *CIDR) UnmarshalText(b []byte) error {
	_, cp, err := net.ParseCIDR(string(b))
	if err != nil {
		return err
	}

	c.IPNet = cp
	return nil
}
