package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cansarihan/fray/internal/engine"
	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/report"
)

func runFuzz(args []string) error {
	flags := flag.NewFlagSet("fuzz", flag.ContinueOnError)
	modelPath := flags.String("model", "", "protocol model file (JSON)")
	targetAddr := flags.String("target", "127.0.0.1:7000", "host:port to fuzz")
	network := flags.String("net", "tcp", "network")
	timeout := flags.Duration("timeout", 2*time.Second, "per-reply timeout; a slower reply counts as a hang")
	allowRemote := flags.Bool("allow-remote", false, "permit a non-loopback target")
	asJSON := flags.Bool("json", false, "print JSON")
	asMarkdown := flags.Bool("md", false, "print a Markdown digest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *modelPath == "" {
		return errors.New("fuzz needs -model")
	}
	if err := guardTarget(*targetAddr, *allowRemote); err != nil {
		return err
	}
	m, err := model.Load(*modelPath)
	if err != nil {
		return err
	}
	ctx, stop := signalContext()
	defer stop()

	last := 0
	emit := func(u engine.Update) {
		if u.Total > 0 && (u.Done-last >= 10 || u.Finding != nil) {
			last = u.Done
			fmt.Fprintf(os.Stderr, "\r%d/%d cases, %d findings ", u.Done, u.Total, 0)
		}
	}
	res, err := engine.Run(ctx, m, engine.Options{Network: *network, Addr: *targetAddr, Timeout: *timeout}, emit)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	switch {
	case *asJSON:
		return report.JSON(os.Stdout, res)
	case *asMarkdown:
		report.Markdown(os.Stdout, res)
	default:
		report.Table(os.Stdout, res)
	}
	return nil
}
