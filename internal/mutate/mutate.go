// Package mutate produces the malformed inputs Fray sends. A mutator takes a
// well-formed value and returns a list of hostile variants of it, each with a
// label that names the idea being tested. Everything is deterministic, so a
// crash can be reproduced exactly.
package mutate

import (
	"fmt"
	"strings"
)

// Variant is one malformed value and the idea behind it.
type Variant struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Mutator turns a seed value into hostile variants.
type Mutator interface {
	Name() string
	Describe() string
	Variants(seed string) []Variant
}

var registry []Mutator

func register(m Mutator) { registry = append(registry, m) }

// Mutators returns the registered mutators in a stable order.
func Mutators() []Mutator {
	out := make([]Mutator, len(registry))
	copy(out, registry)
	return out
}

// Field-level mutators suit "string" and "token" fields.
const (
	KindField = "field"
	KindInt   = "int"
	KindFrame = "frame"
)

// Apply runs every mutator whose kind fits the field type over a seed, de-
// duplicating variants that come out identical.
func Apply(fieldType, seed string) []Variant {
	var kinds []string
	switch fieldType {
	case "int":
		kinds = []string{KindInt}
	default:
		kinds = []string{KindField}
	}
	var out []Variant
	seen := map[string]bool{}
	for _, m := range registry {
		mk, ok := m.(kinded)
		if !ok || !contains(kinds, mk.kind()) {
			continue
		}
		for _, v := range m.Variants(seed) {
			if v.Value == seed || seen[v.Label+"\x00"+v.Value] {
				continue
			}
			seen[v.Label+"\x00"+v.Value] = true
			out = append(out, v)
		}
	}
	return out
}

type kinded interface{ kind() string }

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func repeat(s string, n int) string {
	if s == "" {
		s = "A"
	}
	return strings.Repeat(s, n)
}

func label(format string, args ...any) string { return fmt.Sprintf(format, args...) }
