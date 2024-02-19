// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netif

import (
	"errors"
	"net"
)

// BUG(mikio): On JS, methods and functions related to
// Interface are not implemented.

// BUG(mikio): On AIX, DragonFly BSD, NetBSD, OpenBSD, Plan 9 and
// Solaris, the MulticastAddrs method of Interface is not implemented.

var (
	errInvalidInterface         = errors.New("invalid network interface")
	errInvalidInterfaceIndex    = errors.New("invalid network interface index")
	errInvalidInterfaceName     = errors.New("invalid network interface name")
	errNoSuchInterface          = errors.New("no such network interface")
	errNoSuchMulticastInterface = errors.New("no such multicast network interface")
)

// Interface represents a mapping between network interface name
// and index. It also represents network interface facility
// information.
type Interface struct {
	Index        int              // positive integer that starts at one, zero is never used
	MTU          int              // maximum transmission unit
	Name         string           // e.g., "en0", "lo0", "eth0.100"
	HardwareAddr net.HardwareAddr // IEEE MAC-48, EUI-48 and EUI-64 form
	Flags        InterfaceFlags   // e.g., FlagUp, FlagLoopback, FlagMulticast
}

type InterfaceFlags uint

const (
	FlagUp           InterfaceFlags = 1 << iota // interface is administratively up
	FlagBroadcast                               // interface supports broadcast access capability
	FlagLoopback                                // interface is a loopback interface
	FlagPointToPoint                            // interface belongs to a point-to-point link
	FlagMulticast                               // interface supports multicast access capability
	FlagRunning                                 // interface is in running state
)

var ifFlagNames = []string{
	"up",
	"broadcast",
	"loopback",
	"pointtopoint",
	"multicast",
	"running",
}

func (f InterfaceFlags) String() string {
	s := ""
	for i, name := range ifFlagNames {
		if f&(1<<uint(i)) != 0 {
			if s != "" {
				s += "|"
			}
			s += name
		}
	}
	if s == "" {
		s = "0"
	}
	return s
}

type AddrFlags uint

const (
	FlagDadDuplicated AddrFlags = 1 << iota // address is found duplicated by DAD
	FlagDadTentative                        // address is running DAD
	FlagNoDad                               // address is not tested by DAD
	FlagDeprecated                          // address is deprecated
	FlagTemporary                           // address is temporary
)

var addrFlagNames = []string{
	"duplicated",
	"tentative",
	"no-dad",
	"deprecated",
	"temporary",
}

func (f AddrFlags) String() string {
	s := ""
	for i, name := range addrFlagNames {
		if f&(1<<uint(i)) != 0 {
			if s != "" {
				s += "|"
			}
			s += name
		}
	}
	if s == "" {
		s = "0"
	}
	return s
}

type Addr struct {
	Addr     net.Addr
	Flags    AddrFlags
	RawFlags any
}

// Addrs returns a list of unicast interface addresses for a specific
// interface.
func (ifi *Interface) Addrs() ([]Addr, error) {
	if ifi == nil {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: errInvalidInterface}
	}
	ifat, err := interfaceAddrTable(ifi)
	if err != nil {
		err = &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	return ifat, err
}

// Interfaces returns a list of the system's network interfaces.
func Interfaces() ([]Interface, error) {
	ift, err := interfaceTable(0)
	if err != nil {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	return ift, nil
}

// InterfaceAddrs returns a list of the system's unicast interface
// addresses.
//
// The returned list does not identify the associated interface; use
// Interfaces and [Interface.Addrs] for more detail.
func InterfaceAddrs() ([]Addr, error) {
	ifat, err := interfaceAddrTable(nil)
	if err != nil {
		err = &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	return ifat, err
}

// InterfaceByIndex returns the interface specified by index.
//
// On Solaris, it returns one of the logical network interfaces
// sharing the logical data link; for more precision use
// [InterfaceByName].
func InterfaceByIndex(index int) (*Interface, error) {
	if index <= 0 {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: errInvalidInterfaceIndex}
	}
	ift, err := interfaceTable(index)
	if err != nil {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	ifi, err := interfaceByIndex(ift, index)
	if err != nil {
		err = &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	return ifi, err
}

func interfaceByIndex(ift []Interface, index int) (*Interface, error) {
	for _, ifi := range ift {
		if index == ifi.Index {
			return &ifi, nil
		}
	}
	return nil, errNoSuchInterface
}

// InterfaceByName returns the interface specified by name.
func InterfaceByName(name string) (*Interface, error) {
	if name == "" {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: errInvalidInterfaceName}
	}
	ift, err := interfaceTable(0)
	if err != nil {
		return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: err}
	}
	for _, ifi := range ift {
		if name == ifi.Name {
			return &ifi, nil
		}
	}
	return nil, &net.OpError{Op: "route", Net: "ip+net", Source: nil, Addr: nil, Err: errNoSuchInterface}
}
