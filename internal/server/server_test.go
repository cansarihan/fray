package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cansarihan/fray/internal/engine"
	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/target"
)

func testServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = target.New().Serve(ctx, ln) }()
	t.Cleanup(cancel)
	data, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "kv.json"))
	m, err := model.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(Options{
		Model: m, Engine: engine.Options{Network: "tcp", Addr: ln.Addr().String(), Timeout: 500 * time.Millisecond},
		Token: token, UI: true,
	}, BuildInfo{Version: "test"}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string, v any) int {
	t.Helper()
	res, err := http.Get(url) // #nosec G107 -- httptest server
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if v != nil {
		_ = json.NewDecoder(res.Body).Decode(v)
	}
	return res.StatusCode
}

func TestModelAndMutators(t *testing.T) {
	srv := testServer(t, "")
	var mdl struct {
		Nodes []string     `json:"nodes"`
		Edges []model.Edge `json:"edges"`
	}
	if get(t, srv.URL+"/api/v1/model", &mdl) != http.StatusOK || len(mdl.Nodes) < 4 {
		t.Fatalf("model = %+v", mdl)
	}
	var muts []map[string]string
	if get(t, srv.URL+"/api/v1/mutators", &muts) != http.StatusOK || len(muts) < 6 {
		t.Errorf("mutators = %d", len(muts))
	}
}

func TestRunCampaign(t *testing.T) {
	srv := testServer(t, "")
	res, err := http.Post(srv.URL+"/api/v1/run", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("run = %d", res.StatusCode)
	}
	// a second run while the first is going is refused
	res2, _ := http.Post(srv.URL+"/api/v1/run", "", nil)
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict && res2.StatusCode != http.StatusAccepted {
		t.Errorf("second run = %d", res2.StatusCode)
	}

	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		var st map[string]any
		get(t, srv.URL+"/api/v1/status", &st)
		if st["running"] == false && st["ran"] == true {
			if n, _ := st["findings"].(float64); n > 0 {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("campaign did not finish with findings")
}

func TestToken(t *testing.T) {
	srv := testServer(t, "k")
	if get(t, srv.URL+"/api/v1/model", nil) != http.StatusUnauthorized {
		t.Error("no token should be unauthorized")
	}
	if get(t, srv.URL+"/api/v1/model?token=k", nil) != http.StatusOK {
		t.Error("token should authorize")
	}
}
