package network

import (
	"fmt"
	"net"
	"strconv"
)

// ListenEphemeral binds a TCP listener on host with an OS-assigned free
// port (port 0) and returns the live listener along with the port number
// the kernel picked. This is the genuinely race-free way to obtain a free
// port: unlike PortAllocator.Allocate and serviceregistry.FindAvailablePort
// (both documented check-then-use / TOCTOU helpers — they bind, close, and
// hand back an int that another process can claim before the caller binds
// it again), this function never releases the OS's hold on the port. The
// caller owns the returned listener and is responsible for closing it; the
// port stays reserved for as long as the listener stays open, so it can be
// handed directly to an HTTP server (e.g. `http.Serve(listener, handler)`)
// or any other consumer of a net.Listener without a second bind ever
// happening.
//
// host follows the same convention as the address half of net.Listen's
// "tcp" address argument: "" or "0.0.0.0" binds all interfaces, "127.0.0.1"
// (or any other local address) binds only that interface. This lets a
// caller that needs a specific bind address (not just the wildcard) get a
// race-free ephemeral port on it.
//
// This exists for services — such as a host-networked API process that
// cannot rely on container-runtime port-mapping to avoid collisions with
// unrelated processes on a shared host — that need to claim a free port and
// keep it, rather than merely learn a number that was free a moment ago.
func ListenEphemeral(host string) (net.Listener, int, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, 0, fmt.Errorf("listen on ephemeral port at %q: %w", host, err)
	}

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		return nil, 0, fmt.Errorf("parse assigned port from %q: %w", ln.Addr().String(), err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		_ = ln.Close()
		return nil, 0, fmt.Errorf("convert assigned port %q to int: %w", portStr, err)
	}

	return ln, port, nil
}

// ListenEphemeralPort is a convenience wrapper around ListenEphemeral for
// the common case of binding all interfaces (equivalent to host "" or
// "0.0.0.0"). See ListenEphemeral for the race-free guarantee this provides
// and the caller's responsibility to close the returned listener.
func ListenEphemeralPort() (net.Listener, int, error) {
	return ListenEphemeral("")
}
