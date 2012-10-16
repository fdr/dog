// Ported from some private symbols in the Go stdlib API to deal with
// TCP address parsing issues.  In particular, elide the
// host-name-resolution step, as a dog can currently only be
// instructed to deal with physical IPs.
//
// Virtually all of the code is under this copyright:
//
// Copyright 2010 The Go Authors.  All rights reserved.  Use of this
// source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Any modifications are under this copyright:
//
// Copyright 2012 Heroku.  All rights reserved.
package dogconf

import (
	"errors"
	"net"
)

// Decimal to integer starting at &s[i0].
// Returns number, new offset, success.
func dtoi(s string, i0 int) (n int, i int, ok bool) {
	// Bigger than we need, not too big to worry about overflow
	const big = 0xFFFFFF

	n = 0
	for i = i0; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= big {
			return 0, i, false
		}
	}
	if i == i0 {
		return 0, i, false
	}
	return n, i, true
}

// Convert "host:port" into IP address and port.
func hostPortToAddr(network, hostport string) (a net.Addr, err error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}

	var addr net.IP
	if host != "" {
		// Try as an IP address.
		addr = net.ParseIP(host)
		if addr == nil {
			return nil, errors.New("Invalid IP passed")
		}
	}

	p, i, ok := dtoi(port, 0)
	if !ok || i != len(port) {
		p, err = net.LookupPort(network, port)
		if err != nil {
			return nil, err
		}
	}

	if p < 0 || p > 0xFFFF {
		return nil, &net.AddrError{"invalid port", port}
	}

	return &net.TCPAddr{addr, p}, nil
}
