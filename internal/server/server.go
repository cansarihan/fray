// Package server runs a campaign in the background and exposes its progress and
// findings over an HTTP API, with the panel embedded. One target and one model
// are fixed at startup, so a request can only start, watch or read a run — never
// aim Fray somewhere new.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cansarihan/fray/internal/engine"
	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/mutate"
)

// BuildInfo is the version metadata on the status endpoint.
type BuildInfo struct {
	Version   string
	Commit    string
	GoVersion string
	Platform  string
}

// Options configures the server.
type Options struct {
	Model  *model.Model
	Engine engine.Options
	Token  string
	UI     bool
}

// Server owns one campaign at a time over the fixed model and target.
type Server struct {
	opts    Options
	info    BuildInfo
	started time.Time

	mu       sync.Mutex
	running  bool
	done     int
	total    int
	result   *engine.Result
	findings []engine.Finding
	subs     map[int]chan engine.Update
	seq      int
	ran      bool
}

// New builds a server.
func New(opts Options, info BuildInfo) *Server {
	return &Server{opts: opts, info: info, started: time.Now(), subs: map[int]chan engine.Update{}}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/v1/status", s.guard(s.handleStatus))
	mux.HandleFunc("GET /api/v1/model", s.guard(s.handleModel))
	mux.HandleFunc("GET /api/v1/mutators", s.guard(s.handleMutators))
	mux.HandleFunc("GET /api/v1/findings", s.guard(s.handleFindings))
	mux.HandleFunc("POST /api/v1/run", s.guard(s.handleRun))
	mux.HandleFunc("GET /api/v1/stream", s.guard(s.handleStream))
	if s.opts.UI {
		mux.Handle("GET /", uiHandler())
	}
	return mux
}

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got == "" {
				got = r.URL.Query().Get("token")
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.opts.Token)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := map[string]any{
		"version": s.info.Version, "commit": s.info.Commit, "go_version": s.info.GoVersion, "platform": s.info.Platform,
		"model": s.opts.Model.Name, "target": s.opts.Engine.Addr, "uptime": time.Since(s.started).Round(time.Second).String(),
		"running": s.running, "ran": s.ran, "done": s.done, "total": s.total, "findings": len(s.findings),
	}
	if s.result != nil {
		status["baseline"] = s.result.Baseline
		status["counts"] = s.result.Counts
		status["elapsed"] = s.result.Elapsed
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleModel(w http.ResponseWriter, _ *http.Request) {
	nodes, edges := s.opts.Model.Graph()
	writeJSON(w, http.StatusOK, map[string]any{"name": s.opts.Model.Name, "flows": s.opts.Model.Flows, "nodes": nodes, "edges": edges})
}

func (s *Server) handleMutators(w http.ResponseWriter, _ *http.Request) {
	type m struct {
		Name, Describe string
	}
	out := []m{}
	for _, mu := range mutate.Mutators() {
		out = append(out, m{mu.Name(), mu.Describe()})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFindings(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := append([]engine.Finding(nil), s.findings...)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRun(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a campaign is already running"})
		return
	}
	s.running = true
	s.ran = true
	s.findings = nil
	s.result = nil
	s.done, s.total = 0, 0
	s.mu.Unlock()
	go s.campaign()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (s *Server) campaign() {
	res, err := engine.Run(context.Background(), s.opts.Model, s.opts.Engine, s.publish)
	s.mu.Lock()
	s.running = false
	if err == nil {
		s.result = &res
	} else {
		s.result = &engine.Result{Model: s.opts.Model.Name, Counts: map[string]int{}}
	}
	s.mu.Unlock()
	s.publish(engine.Update{Done: s.total, Total: s.total}) // flush
	s.broadcastDone(err)
}

func (s *Server) publish(u engine.Update) {
	s.mu.Lock()
	if u.Total > 0 {
		s.total = u.Total
	}
	s.done = u.Done
	if u.Finding != nil {
		s.findings = append(s.findings, *u.Finding)
	}
	targets := make([]chan engine.Update, 0, len(s.subs))
	for _, ch := range s.subs {
		targets = append(targets, ch)
	}
	s.mu.Unlock()
	for _, ch := range targets {
		select {
		case ch <- u:
		default:
		}
	}
}

func (s *Server) broadcastDone(err error) {
	msg := engine.Update{Done: s.done, Total: s.total}
	if err != nil {
		msg.Finding = &engine.Finding{Outcome: "error", Reply: err.Error()}
	}
	s.publish(msg)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := make(chan engine.Update, 256)
	s.mu.Lock()
	s.seq++
	id := s.seq
	s.subs[id] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}()

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case u := <-ch:
			data, _ := json.Marshal(u)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-beat.C:
			_, _ = fmt.Fprint(w, ": keep alive\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
