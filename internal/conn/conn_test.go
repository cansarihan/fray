package conn

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/target"
)

const kv = `{"name":"kv","banner":"^\\+OK","flows":[{"name":"store","steps":[
  {"name":"auth","send":"AUTH {u}","expect":"^\\+OK","fields":{"u":{"type":"token","seed":"demo"}}},
  {"name":"set","send":"SET {k} {v}","expect":"^\\+OK","fields":{"k":{"type":"token","seed":"k1"},"v":{"type":"string","seed":"hi"}}}
]}]}`

func serveTarget(t *testing.T) (string, *model.Model) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = target.New().Serve(ctx, ln) }()
	t.Cleanup(cancel)
	m, err := model.Parse([]byte(kv))
	if err != nil {
		t.Fatal(err)
	}
	return ln.Addr().String(), m
}

func TestSessionHappyPath(t *testing.T) {
	addr, m := serveTarget(t)
	s, banner, err := Dial("tcp", addr, m, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if banner != "+OK kvd ready" {
		t.Errorf("banner = %q", banner)
	}
	auth := &m.Flows[0].Steps[0]
	if r := s.Send(auth.Render(nil, m.Terminator), auth); r.Outcome != OK {
		t.Errorf("auth outcome = %s (%v)", r.Outcome, r.Err)
	}
	set := &m.Flows[0].Steps[1]
	if r := s.Send(set.Render(nil, m.Terminator), set); r.Outcome != OK {
		t.Errorf("set outcome = %s", r.Outcome)
	}
}

func TestMismatchAndClose(t *testing.T) {
	addr, m := serveTarget(t)
	s, _, err := Dial("tcp", addr, m, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	set := &m.Flows[0].Steps[1]
	// SET with no value crashes the handler, which drops the connection.
	r := s.Send("SET onlykey\r\n", set)
	if r.Outcome != Closed && r.Outcome != Mismatch {
		t.Errorf("missing-arg outcome = %s, want closed or mismatch", r.Outcome)
	}
}

func TestDialRefused(t *testing.T) {
	m, _ := model.Parse([]byte(kv))
	if _, _, err := Dial("tcp", "127.0.0.1:1", m, 500*time.Millisecond); err == nil {
		t.Error("dialing a closed port should fail")
	}
}
