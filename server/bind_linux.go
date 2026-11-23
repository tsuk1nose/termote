//go:build linux

package main

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// listenTCP binds a TCP listener. On Linux it enables IP_FREEBIND /
// IPV6_FREEBIND so binding a specific non-loopback address (such as a
// Tailscale IP) succeeds even if the interface comes up after termote starts.
func listenTCP(ctx context.Context, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			_ = c.Control(func(fd uintptr) {
				_ = unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_FREEBIND, 1)
				_ = unix.SetsockoptInt(int(fd), unix.SOL_IPV6, unix.IPV6_FREEBIND, 1)
			})
			return nil
		},
	}
	return lc.Listen(ctx, "tcp", addr)
}
