// Package target is a small key-value server used as Fray's demo subject. It
// speaks the protocol in testdata/kv.json and carries a few deliberate bugs:
// an oversized value overruns a fixed buffer, a negative TTL panics, and a
// missing argument is dereferenced. Fray is meant to find these.
package target

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server is the demo key-value service.
type Server struct {
	mu    sync.Mutex
	store map[string]string
}

// New returns an empty server.
func New() *Server { return &Server{store: map[string]string{}} }

// Serve answers connections until ctx ends.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() { <-ctx.Done(); _ = ln.Close() }()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer func() {
		// A panic in a handler takes the whole process down in a naive server;
		// here it only drops the connection, which Fray sees as a reset.
		_ = recover()
		_ = c.Close()
	}()
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))
	_, _ = io.WriteString(c, "+OK kvd ready\r\n")
	r := bufio.NewReader(c)
	authed := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		reply := s.exec(strings.Fields(strings.TrimRight(line, "\r\n")), &authed)
		if _, err := io.WriteString(c, reply+"\r\n"); err != nil {
			return
		}
	}
}

func (s *Server) exec(args []string, authed *bool) string {
	if len(args) == 0 {
		return "-ERR empty"
	}
	switch strings.ToUpper(args[0]) {
	case "AUTH":
		*authed = true
		return "+OK"
	case "QUIT":
		return "+BYE"
	case "SET":
		// BUG: assumes a value argument exists, so SET with a missing value
		// dereferences past the end of args and drops the connection.
		value := args[2]
		s.mu.Lock()
		s.store[args[1]] = value
		s.mu.Unlock()
		return "+OK"
	case "GET":
		s.mu.Lock()
		v, ok := s.store[args[1]]
		s.mu.Unlock()
		if !ok {
			return "-ERR no such key"
		}
		return "+" + v
	case "EXPIRE":
		// BUG: the TTL indexes a fixed ring with no bounds check, so a negative
		// or out-of-range value panics and drops the connection.
		ttl, _ := strconv.Atoi(args[2])
		var ring [3600]int64
		ring[ttl]++
		return "+OK"
	default:
		return fmt.Sprintf("-ERR unknown command %q", args[0])
	}
}
