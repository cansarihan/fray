package model

import (
	"strings"
	"testing"
)

const sample = `{
  "name": "kv",
  "banner": "^\\+OK",
  "flows": [
    {"name": "store", "steps": [
      {"name": "auth", "send": "AUTH {user}", "expect": "^\\+OK", "fields": {"user": {"type": "token", "seed": "demo"}}},
      {"name": "set", "send": "SET {key} {value}", "expect": "^\\+OK", "fields": {
        "key": {"type": "token", "seed": "k1"}, "value": {"type": "string", "seed": "hello"}}},
      {"name": "quit", "send": "QUIT", "expect": "^\\+BYE"}
    ]},
    {"name": "read", "steps": [
      {"name": "auth", "send": "AUTH {user}", "expect": "^\\+OK", "fields": {"user": {"type": "token", "seed": "demo"}}},
      {"name": "get", "send": "GET {key}", "expect": "^(\\+|-ERR)", "fields": {"key": {"type": "token", "seed": "k1"}}}
    ]}
  ]
}`

func TestParseAndRender(t *testing.T) {
	m, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if m.Terminator != "\r\n" || !m.BannerOK("+OK kvd ready") || m.BannerOK("-ERR") {
		t.Errorf("model defaults wrong: %+v", m)
	}
	set := &m.Flows[0].Steps[1]
	if got := set.Render(map[string]string{"value": "XYZ"}, m.Terminator); got != "SET k1 XYZ\r\n" {
		t.Errorf("render = %q", got)
	}
	if !set.Matches("+OK stored") || set.Matches("-ERR") {
		t.Error("reply matching wrong")
	}
	if names := set.FieldNames(); strings.Join(names, ",") != "key,value" {
		t.Errorf("field names = %v", names)
	}
}

func TestGraph(t *testing.T) {
	m, _ := Parse([]byte(sample))
	nodes, edges := m.Graph()
	if strings.Join(nodes, ",") != "connect,auth,set,quit,get" {
		t.Errorf("nodes = %v", nodes)
	}
	if len(edges) != 4 { // connect→auth shared by both flows
		t.Errorf("edges = %v", edges)
	}
}

func TestParseRejects(t *testing.T) {
	bad := map[string]string{
		"no flows":     `{"name":"x","flows":[]}`,
		"bad pattern":  `{"flows":[{"name":"f","steps":[{"name":"s","send":"X","expect":"("}]}]}`,
		"undeclared":   `{"flows":[{"name":"f","steps":[{"name":"s","send":"X {y}","expect":"."}]}]}`,
		"unknown type": `{"flows":[{"name":"f","steps":[{"name":"s","send":"X {y}","expect":".","fields":{"y":{"type":"float"}}}]}]}`,
		"not json":     `flows:`,
	}
	for name, src := range bad {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
