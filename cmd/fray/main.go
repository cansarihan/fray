// Command fray exercises one of your own network services against a protocol
// model: it mutates each field and frame and reports where the server hangs,
// resets or breaks the protocol. The target defaults to loopback; reaching
// another host needs -allow-remote.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fray: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return usage()
	}
	args := os.Args[2:]
	switch os.Args[1] {
	case "fuzz":
		return runFuzz(args)
	case "model":
		return runModel(args)
	case "mutators":
		return runMutators(args)
	case "target":
		return runTarget(args)
	case "serve":
		return runServe(args)
	case "version":
		fmt.Printf("fray %s (commit %s, built %s)\n", version, commit, date)
		return nil
	default:
		return usage()
	}
}

func usage() error {
	fmt.Fprintln(os.Stderr, "usage: fray <fuzz|model|mutators|target|serve|version> [flags]")
	fmt.Fprintln(os.Stderr, "  fray fuzz -model kv.json -target 127.0.0.1:7000")
	fmt.Fprintln(os.Stderr, "  fray target -listen 127.0.0.1:7000      # the demo service")
	fmt.Fprintln(os.Stderr, "  fray serve -model kv.json -target 127.0.0.1:7000")
	return errors.New("no command given")
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// guardTarget refuses a non-loopback target unless the operator opted in. Fray
// sends malformed traffic, so it must not be aimed at a host by accident.
func guardTarget(addr string, allowRemote bool) error {
	if allowRemote {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("target %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	return fmt.Errorf("target %s is not loopback; pass -allow-remote to fuzz a host you are authorized to test", addr)
}
