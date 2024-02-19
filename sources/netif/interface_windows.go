// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netif

import (
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	IpAdapterAddressDnsEligible uint32 = 1 << iota
	IpAdapterAddressTransient
)

const (
	IpDadStateInvalid int32 = iota
	IpDadStateTentative
	IpDadStateDuplicate
	IpDadStateDeprecated
	IpDadStatePreferred
)

// adapterAddresses returns a list of IP adapter and address
// structures. The structure contains an IP adapter and flattened
// multiple IP addresses including unicast, anycast and multicast
// addresses.
func adapterAddresses() ([]*windows.IpAdapterAddresses, error) {
	var b []byte
	l := uint32(15000) // recommended initial size
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return nil, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
	}
	var aas []*windows.IpAdapterAddresses
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		aas = append(aas, aa)
	}
	return aas, nil
}

// If the ifindex is zero, interfaceTable returns mappings of all
// network interfaces. Otherwise it returns a mapping of a specific
// interface.
func interfaceTable(ifindex int) ([]Interface, error) {
	aas, err := adapterAddresses()
	if err != nil {
		return nil, err
	}
	var ift []Interface
	for _, aa := range aas {
		index := aa.IfIndex
		if index == 0 { // ipv6IfIndex is a substitute for ifIndex
			index = aa.Ipv6IfIndex
		}
		if ifindex == 0 || ifindex == int(index) {
			ifi := Interface{
				Index: int(index),
				Name:  windows.UTF16PtrToString(aa.FriendlyName),
			}
			if aa.OperStatus == windows.IfOperStatusUp {
				ifi.Flags |= FlagUp
				ifi.Flags |= FlagRunning
			}
			// For now we need to infer link-layer service
			// capabilities from media types.
			// TODO: use MIB_IF_ROW2.AccessType now that we no longer support
			// Windows XP.
			switch aa.IfType {
			case windows.IF_TYPE_ETHERNET_CSMACD, windows.IF_TYPE_ISO88025_TOKENRING, windows.IF_TYPE_IEEE80211, windows.IF_TYPE_IEEE1394:
				ifi.Flags |= FlagBroadcast | FlagMulticast
			case windows.IF_TYPE_PPP, windows.IF_TYPE_TUNNEL:
				ifi.Flags |= FlagPointToPoint | FlagMulticast
			case windows.IF_TYPE_SOFTWARE_LOOPBACK:
				ifi.Flags |= FlagLoopback | FlagMulticast
			case windows.IF_TYPE_ATM:
				ifi.Flags |= FlagBroadcast | FlagPointToPoint | FlagMulticast // assume all services available; LANE, point-to-point and point-to-multipoint
			}
			if aa.Mtu == 0xffffffff {
				ifi.MTU = -1
			} else {
				ifi.MTU = int(aa.Mtu)
			}
			if aa.PhysicalAddressLength > 0 {
				ifi.HardwareAddr = make(net.HardwareAddr, aa.PhysicalAddressLength)
				copy(ifi.HardwareAddr, aa.PhysicalAddress[:])
			}
			ift = append(ift, ifi)
			if ifindex == ifi.Index {
				break
			}
		}
	}
	return ift, nil
}

// If the ifi is nil, interfaceAddrTable returns addresses for all
// network interfaces. Otherwise it returns addresses for a specific
// interface.
func interfaceAddrTable(ifi *Interface) ([]Addr, error) {
	aas, err := adapterAddresses()
	if err != nil {
		return nil, err
	}
	var ifat []Addr
	for _, aa := range aas {
		index := aa.IfIndex
		if index == 0 { // ipv6IfIndex is a substitute for ifIndex
			index = aa.Ipv6IfIndex
		}
		if ifi == nil || ifi.Index == int(index) {
			for puni := aa.FirstUnicastAddress; puni != nil; puni = puni.Next {
				sa, err := puni.Address.Sockaddr.Sockaddr()
				if err != nil {
					return nil, os.NewSyscallError("sockaddr", err)
				}
				addr := Addr{Flags: FlagTemporary, RawFlags: []any{puni.Flags, puni.DadState}}
				if puni.Flags&IpAdapterAddressDnsEligible != 0 {
					addr.Flags &= ^FlagTemporary
				}

				switch puni.DadState {
				case IpDadStateInvalid:
					addr.Flags |= FlagNoDad
				case IpDadStateTentative:
					addr.Flags |= FlagDadTentative
				case IpDadStateDeprecated:
					addr.Flags |= FlagDeprecated
				case IpDadStateDuplicate:
					addr.Flags |= FlagDadDuplicated
				case IpDadStatePreferred:
					// nothing to do
				}

				switch sa := sa.(type) {
				case *syscall.SockaddrInet4:
					addr.Addr = &net.IPNet{IP: net.IPv4(sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3]), Mask: net.CIDRMask(int(puni.OnLinkPrefixLength), 8*net.IPv4len)}
					ifat = append(ifat)
				case *syscall.SockaddrInet6:
					ifa := &net.IPNet{IP: make(net.IP, net.IPv6len), Mask: net.CIDRMask(int(puni.OnLinkPrefixLength), 8*net.IPv6len)}
					copy(ifa.IP, sa.Addr[:])
					addr.Addr = ifa
				}
				ifat = append(ifat, addr)
			}
			for pany := aa.FirstAnycastAddress; pany != nil; pany = pany.Next {
				sa, err := pany.Address.Sockaddr.Sockaddr()
				if err != nil {
					return nil, os.NewSyscallError("sockaddr", err)
				}
				switch sa := sa.(type) {
				case *syscall.SockaddrInet4:
					ifat = append(ifat, Addr{Addr: &net.IPAddr{IP: net.IPv4(sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3])}})
				case *syscall.SockaddrInet6:
					ifa := &net.IPAddr{IP: make(net.IP, net.IPv6len)}
					copy(ifa.IP, sa.Addr[:])
					ifat = append(ifat, Addr{Addr: ifa})
				}
			}
		}
	}
	return ifat, nil
}

// interfaceMulticastAddrTable returns addresses for a specific
// interface.
func interfaceMulticastAddrTable(ifi *Interface) ([]Addr, error) {
	aas, err := adapterAddresses()
	if err != nil {
		return nil, err
	}
	var ifat []Addr
	for _, aa := range aas {
		index := aa.IfIndex
		if index == 0 { // ipv6IfIndex is a substitute for ifIndex
			index = aa.Ipv6IfIndex
		}
		if ifi == nil || ifi.Index == int(index) {
			for pmul := aa.FirstMulticastAddress; pmul != nil; pmul = pmul.Next {
				sa, err := pmul.Address.Sockaddr.Sockaddr()
				if err != nil {
					return nil, os.NewSyscallError("sockaddr", err)
				}
				switch sa := sa.(type) {
				case *syscall.SockaddrInet4:
					ifat = append(ifat, Addr{Addr: &net.IPAddr{IP: net.IPv4(sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3])}})
				case *syscall.SockaddrInet6:
					ifa := &net.IPAddr{IP: make(net.IP, net.IPv6len)}
					copy(ifa.IP, sa.Addr[:])
					ifat = append(ifat, Addr{Addr: ifa})
				}
			}
		}
	}
	return ifat, nil
}
