// Package conn speaks a model's protocol to a target over one short-lived TCP
// connection. It reads the banner, sends each step, and reads one line of reply,
// all under a deadline and a read cap. It never interprets a reply as anything
// but bytes to match against the model's pattern.
package conn

import (
	"bufio"
	"errors"
	"io"
	"net"
	"time"

	"github.com/cansarihan/fray/internal/model"
)

const maxLine = 1 << 20

// Outcome is how a single send-and-reply turn ended.
type Outcome string

const (
	// OK means the reply arrived and matched the step's pattern.
	OK Outcome = "ok"
	// Mismatch means a reply arrived but did not match.
	Mismatch Outcome = "mismatch"
	// Closed means the peer closed the connection instead of replying.
	Closed Outcome = "closed"
	// Timeout means no reply arrived in time — a possible hang.
	Timeout Outcome = "timeout"
	// Error means the connection failed for another reason.
	Error Outcome = "error"
)

// Reply is the result of one turn.
type Reply struct {
	Outcome Outcome
	Line    string
	Err     error
}

// Session is one open connection to a target.
type Session struct {
	conn    net.Conn
	reader  *bufio.Reader
	timeout time.Duration
	term    string
}

// Dial opens a session and reads the banner, checking it against the model.
func Dial(network, addr string, m *model.Model, timeout time.Duration) (*Session, string, error) {
	c, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return nil, "", err
	}
	s := &Session{conn: c, reader: bufio.NewReaderSize(c, 4096), timeout: timeout, term: m.Terminator}
	banner := ""
	if m.Banner != "" {
		reply := s.read()
		if reply.Err != nil {
			_ = c.Close()
			return nil, "", reply.Err
		}
		banner = reply.Line
	}
	return s, banner, nil
}

// Send writes a raw message (already terminated) and reads one reply, judging
// it against the step.
func (s *Session) Send(raw string, step *model.Step) Reply {
	_ = s.conn.SetDeadline(time.Now().Add(s.timeout))
	if _, err := io.WriteString(s.conn, raw); err != nil {
		return classifyWrite(err)
	}
	reply := s.read()
	if reply.Outcome != OK {
		return reply
	}
	if step != nil && !step.Matches(reply.Line) {
		reply.Outcome = Mismatch
	}
	return reply
}

func (s *Session) read() Reply {
	_ = s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	line, err := s.reader.ReadString('\n')
	switch {
	case err == nil:
		return Reply{Outcome: OK, Line: trim(line)}
	case len(line) >= maxLine:
		return Reply{Outcome: OK, Line: trim(line[:maxLine])}
	case errors.Is(err, io.EOF):
		if line != "" {
			return Reply{Outcome: OK, Line: trim(line)}
		}
		return Reply{Outcome: Closed}
	case isTimeout(err):
		return Reply{Outcome: Timeout, Err: err}
	default:
		return Reply{Outcome: Error, Err: err}
	}
}

// Close ends the session.
func (s *Session) Close() { _ = s.conn.Close() }

func classifyWrite(err error) Reply {
	if isTimeout(err) {
		return Reply{Outcome: Timeout, Err: err}
	}
	return Reply{Outcome: Closed, Err: err}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
