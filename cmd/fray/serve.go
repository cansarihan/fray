package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/cansarihan/fray/internal/engine"
	"github.com/cansarihan/fray/internal/logging"
	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/server"
)

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	modelPath := flags.String("model", "", "protocol model file")
	targetAddr := flags.String("target", "127.0.0.1:7000", "host:port to fuzz")
	network := flags.String("net", "tcp", "network")
	timeout := flags.Duration("timeout", 2*time.Second, "per-reply timeout")
	allowRemote := flags.Bool("allow-remote", false, "permit a non-loopback target")
	panel := flags.String("panel", "127.0.0.1:9100", "panel address")
	token := flags.String("token", os.Getenv("FRAY_TOKEN"), "require this token on the panel")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *modelPath == "" {
		return errors.New("serve needs -model")
	}
	if err := guardTarget(*targetAddr, *allowRemote); err != nil {
		return err
	}
	m, err := model.Load(*modelPath)
	if err != nil {
		return err
	}
	log := logging.New("info", "text", os.Stderr)
	ctx, stop := signalContext()
	defer stop()

	handler := server.New(server.Options{
		Model: m, Engine: engine.Options{Network: *network, Addr: *targetAddr, Timeout: *timeout}, Token: *token, UI: true,
	}, server.BuildInfo{Version: version, Commit: commit, GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH}).Handler()

	srv := &http.Server{Addr: *panel, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Info("panel listening", "addr", *panel, "model", m.Name, "target", *targetAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("panel stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
