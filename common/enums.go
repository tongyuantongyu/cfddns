package common

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Family int

const (
	IPv4 Family = iota
	IPv6
)

func (f *Family) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "4", "v4", "ipv4":
		*f = IPv4
	case "6", "v6", "ipv6":
		*f = IPv6
	default:
		return errors.New("invalid IP family")
	}
	return nil
}

func (f *Family) String() string {
	if f == nil {
		return "*"
	}
	switch *f {
	case IPv4:
		return "IPv4"
	case IPv6:
		return "IPv6"
	default:
		return fmt.Sprintf("unknown<%d>", int(*f))
	}
}

type IPSelectMode int

const (
	SelectFirst IPSelectMode = iota
	SelectShortest
	SelectLast
)

func (m *IPSelectMode) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "first":
		*m = SelectFirst
	case "shortest":
		*m = SelectShortest
	case "last":
		*m = SelectLast
	default:
		return errors.New("invalid mode")
	}
	return nil
}

func (m IPSelectMode) String() string {
	switch m {
	case SelectFirst:
		return "first"
	case SelectShortest:
		return "shortest"
	case SelectLast:
		return "last"
	default:
		return fmt.Sprintf("unknown<%d>", int(m))
	}
}

type IPFilterFlag uint64

const (
	FlagNonGlobalUnicast IPFilterFlag = 1 << iota
	FlagPrivate
	FlagNoEUI64
	FlagTemporary
	FlagBadDad
	FlagDeprecated
)

func (f *IPFilterFlag) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "nonglobalunicast", "non-global-unicast", "allow-non-global-unicast":
		*f = FlagNonGlobalUnicast
	case "private", "allow-private":
		*f = FlagPrivate
	case "noeui64", "no-eui64", "excludeeui64", "exclude-eui64":
		*f = FlagNoEUI64
	case "temporary", "allowtemporary", "allow-temporary":
		*f = FlagTemporary
	case "baddad", "allowbaddad", "bad-dad", "allow-bad-dad":
		*f = FlagBadDad
	case "deprecated", "allowdeprecated", "allow-deprecated":
		*f = FlagDeprecated
	default:
		return errors.New("invalid mode")
	}
	return nil
}

func (f IPFilterFlag) String() string {
	flags := ""
	if f.Match(FlagNonGlobalUnicast) {
		flags += ",allow-non-global-unicast"
	}
	if f.Match(FlagPrivate) {
		flags += ",allow-private"
	}
	if f.Match(FlagNoEUI64) {
		flags += ",no-eui64"
	}
	if f.Match(FlagTemporary) {
		flags += ",allow-temporary"
	}
	if f.Match(FlagBadDad) {
		flags += ",allow-bad-dad"
	}
	if f.Match(FlagDeprecated) {
		flags += ",allow-deprecated"
	}

	if flags == "" {
		return strconv.FormatUint(uint64(f), 16)
	} else {
		return fmt.Sprintf("%x(%s)", uint64(f), flags[1:])
	}
}

func (f IPFilterFlag) Match(l IPFilterFlag) bool {
	return (f & l) != 0
}
