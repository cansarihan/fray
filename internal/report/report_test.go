package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cansarihan/fray/internal/engine"
)

func sample() engine.Result {
	return engine.Result{
		Model: "kv", Target: "127.0.0.1:7000", Cases: 79, Elapsed: 2 * time.Second,
		Counts: map[string]int{"closed": 2, "timeout": 1},
		Findings: []engine.Finding{
			{Flow: "store", Step: "set", Target: "field:value", Mutation: "empty", Outcome: "closed", Severity: 2, Repro: []string{"AUTH demo\r\n", "SET k1 \r\n"}},
			{Flow: "store", Step: "set", Target: "frame", Mutation: "no terminator", Outcome: "timeout", Severity: 3, Repro: []string{"AUTH demo\r\n", "SET k1 hello"}},
		},
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, sample())
	out := buf.String()
	for _, want := range []string{"kv against 127.0.0.1:7000", "1 hang, 2 reset", "hang", "reset", "field:value"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q\n%s", want, out)
		}
	}
	// the hang (severity 3) is listed before the reset (severity 2)
	if strings.Index(out, "hang") > strings.Index(out, "reset") {
		t.Error("findings not ordered by severity")
	}
}

func TestMarkdownReplay(t *testing.T) {
	var buf bytes.Buffer
	Markdown(&buf, sample())
	out := buf.String()
	if !strings.Contains(out, "## Hang (1)") || !strings.Contains(out, "SET k1 hello") {
		t.Errorf("markdown missing replay\n%s", out)
	}
	if !strings.Contains(out, "AUTH demo\\r\\n") {
		t.Error("control bytes not escaped in replay")
	}
}

func TestEmpty(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, engine.Result{Model: "kv", Cases: 10})
	if !strings.Contains(buf.String(), "withstood every mutation") {
		t.Error("empty result should say so")
	}
}
