package main

import (
	"flag"
	"net"

	"os"

	"github.com/cansarihan/fray/internal/logging"
	"github.com/cansarihan/fray/internal/target"
)

func runTarget(args []string) error {
	flags := flag.NewFlagSet("target", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:7000", "address for the demo key-value server")
	if err := flags.Parse(args); err != nil {
		return err
	}
	log := logging.New("info", "text", os.Stderr)
	ctx, stop := signalContext()
	defer stop()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	log.Info("demo target listening", "addr", ln.Addr().String())
	return target.New().Serve(ctx, ln)
}
