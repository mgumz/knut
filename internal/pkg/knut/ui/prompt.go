// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package ui

import (
	"net"
	"net/netip"
	"slices"
)

func PromptBindAddr(netAddrs []net.Addr) (string, error) {

	addrs := []string{}
	for i := range netAddrs {
		prefix, _ := netip.ParsePrefix(netAddrs[i].String())
		addr := prefix.Addr()
		if addr.IsLoopback() || addr.IsMulticast() || addr.IsInterfaceLocalMulticast() {
			continue
		}
		addrs = append(addrs, addr.String())
	}

	slices.Sort(addrs)

	return promptHuh(addrs)
}
