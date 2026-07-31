// Package listen resolves the address a local koment server binds to. It is
// shared by the MCP server and the web UI so the two cannot drift apart on the
// one decision that has a security consequence.
package listen

import (
	"errors"
	"fmt"
	"io"
	"net"
)

const loopback = "127.0.0.1"

// Address fills in a missing host with the loopback interface, so that a bare
// port never publishes a repository to the local network.
func Address(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		bare, bareErr := barePort(address)
		if bareErr != nil {
			return "", fmt.Errorf("%q is not a valid address or port: %w", address, err)
		}
		return net.JoinHostPort(loopback, bare), nil
	}

	if host == "" {
		return net.JoinHostPort(loopback, port), nil
	}
	return address, nil
}

func barePort(address string) (string, error) {
	if address == "" {
		return "", errors.New("no port given")
	}
	if _, err := net.LookupPort("tcp", address); err != nil {
		return "", err
	}
	return address, nil
}

// WarnIfPublic says so, loudly, when a bind address is reachable from beyond
// this machine. Neither server authenticates.
func WarnIfPublic(address string, stderr io.Writer) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return
	}
	if parsed := net.ParseIP(host); parsed != nil && parsed.IsLoopback() {
		return
	}
	if host == "localhost" {
		return
	}

	fmt.Fprintf(stderr,
		"koment: WARNING serving on %s, which is not loopback. There is no authentication; "+
			"anyone who can reach this port can read every annotation in the repository.\n", address)
}
