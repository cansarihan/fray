package mutate

import (
	"strings"
	"testing"
)

func labels(vs []Variant) map[string]bool {
	out := map[string]bool{}
	for _, v := range vs {
		out[v.Label] = true
	}
	return out
}

func TestApplyStringField(t *testing.T) {
	vs := Apply("string", "hello")
	if len(vs) < 10 {
		t.Fatalf("string field produced only %d variants", len(vs))
	}
	l := labels(vs)
	for _, want := range []string{"empty", "1 KiB", "printf", "quote", "control range"} {
		if !l[want] {
			t.Errorf("missing variant %q", want)
		}
	}
	// no int-only variants leak into a string field
	if l["int32 max + 1"] {
		t.Error("integer variant applied to a string field")
	}
	// the seed itself is never returned as a variant
	for _, v := range vs {
		if v.Value == "hello" {
			t.Error("seed returned unchanged")
		}
	}
}

func TestApplyIntField(t *testing.T) {
	l := labels(Apply("int", "42"))
	for _, want := range []string{"negative", "overflow", "not a number", "int64 min"} {
		if !l[want] {
			t.Errorf("missing int variant %q", want)
		}
	}
	if l["printf"] {
		t.Error("string mutator applied to an int field")
	}
}

func TestFrame(t *testing.T) {
	vs := Frame("SET k v\r\n")
	l := labels(vs)
	for _, want := range []string{"no terminator", "doubled terminator", "smuggled command"} {
		if !l[want] {
			t.Errorf("missing framing variant %q", want)
		}
	}
	for _, v := range vs {
		if v.Label == "no terminator" && strings.HasSuffix(v.Value, "\n") {
			t.Error("no-terminator variant still ends in a newline")
		}
		if v.Label == "smuggled command" && !strings.Contains(v.Value, "SMUGGLED") {
			t.Error("smuggled variant missing its payload")
		}
	}
}

func TestMutatorsDocumented(t *testing.T) {
	if len(Mutators()) < 6 {
		t.Fatalf("only %d mutators", len(Mutators()))
	}
	for _, m := range Mutators() {
		if m.Name() == "" || m.Describe() == "" {
			t.Errorf("mutator %T is undocumented", m)
		}
	}
}
