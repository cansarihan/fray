package engine

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/target"
)

func runAgainstTarget(t *testing.T, emit func(Update)) Result {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = target.New().Serve(ctx, ln) }()
	t.Cleanup(cancel)

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kv.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), m, Options{Network: "tcp", Addr: ln.Addr().String(), Timeout: 700 * time.Millisecond}, emit)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestRunFindsBugs(t *testing.T) {
	var updates int
	res := runAgainstTarget(t, func(Update) { updates++ })
	if !res.Baseline {
		t.Fatal("baseline should pass against the demo target")
	}
	if res.Cases < 50 || len(res.Findings) == 0 {
		t.Fatalf("cases=%d findings=%d", res.Cases, len(res.Findings))
	}
	if updates == 0 {
		t.Error("no progress updates emitted")
	}
	// the missing-value crash: SET with an empty value drops the connection
	found := false
	for _, f := range res.Findings {
		if f.Step == "set" && f.Target == "field:value" && f.Mutation == "empty" && f.Outcome == "closed" {
			found = true
			if len(f.Repro) < 2 {
				t.Errorf("finding should carry a repro: %+v", f)
			}
		}
	}
	if !found {
		t.Errorf("expected the empty-value crash; findings = %+v", res.Findings)
	}
}

func TestBaselineFailsWithoutTarget(t *testing.T) {
	data, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "kv.json"))
	m, _ := model.Parse(data)
	_, err := Run(context.Background(), m, Options{Network: "tcp", Addr: "127.0.0.1:1", Timeout: 300 * time.Millisecond}, nil)
	if err == nil {
		t.Fatal("a run against a closed port should fail at the baseline")
	}
}
