// Package engine runs a model against a target: first the baseline flows to be
// sure the protocol works, then one mutation at a time — each hostile value or
// broken frame sent in place of a well-formed one — recording every case where
// the server hangs, resets or stops matching the protocol.
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/cansarihan/fray/internal/model"
	"github.com/cansarihan/fray/internal/mutate"
)

// Options configures a run.
type Options struct {
	Network string
	Addr    string
	Timeout time.Duration
}

// Finding is one case worth a human's attention.
type Finding struct {
	ID       int            `json:"id"`
	Flow     string         `json:"flow"`
	Step     string         `json:"step"`
	Target   string         `json:"target"` // "field:<name>" or "frame"
	Mutation string         `json:"mutation"`
	Outcome  string         `json:"outcome"`
	Sent     string         `json:"sent"`
	Reply    string         `json:"reply,omitempty"`
	Repro    []string       `json:"repro"` // every line sent, for replay
	Severity int            `json:"severity"`
	Fields   map[string]any `json:"fields,omitempty"`
}

// Update streams progress.
type Update struct {
	Done    int      `json:"done"`
	Total   int      `json:"total"`
	Finding *Finding `json:"finding,omitempty"`
}

// Result is the whole run.
type Result struct {
	Model    string         `json:"model"`
	Target   string         `json:"target"`
	Cases    int            `json:"cases"`
	Baseline bool           `json:"baseline"`
	Elapsed  time.Duration  `json:"elapsed"`
	Findings []Finding      `json:"findings"`
	Counts   map[string]int `json:"counts"`
}

type testCase struct {
	flow     *model.Flow
	stepIdx  int
	target   string
	mutation string
	// overrides for the mutated step
	fieldName   string
	fieldValue  string
	rawOverride string // when set, replaces the rendered message (framing)
}

// Run executes the baseline and every mutation case, streaming updates.
func Run(ctx context.Context, m *model.Model, opts Options, emit func(Update)) (Result, error) {
	start := time.Now()
	res := Result{Model: m.Name, Target: opts.Addr, Counts: map[string]int{}}

	ok, err := baseline(ctx, m, opts)
	if err != nil {
		return res, err
	}
	res.Baseline = ok
	if !ok {
		return res, fmt.Errorf("baseline failed: the target does not speak this protocol cleanly")
	}

	cases := plan(m)
	res.Cases = len(cases)
	nextID := 0
	for i, tc := range cases {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if f := runCase(m, opts, tc); f != nil {
			nextID++
			f.ID = nextID
			res.Findings = append(res.Findings, *f)
			res.Counts[f.Outcome]++
			if emit != nil {
				clone := *f
				emit(Update{Done: i + 1, Total: len(cases), Finding: &clone})
				continue
			}
		}
		if emit != nil && (i+1)%25 == 0 {
			emit(Update{Done: i + 1, Total: len(cases)})
		}
	}
	res.Elapsed = time.Since(start).Round(time.Millisecond)
	return res, nil
}

// plan builds every mutation case: one per (step, field, variant) and one per
// (step, framing variant).
func plan(m *model.Model) []testCase {
	var cases []testCase
	for fi := range m.Flows {
		flow := &m.Flows[fi]
		for si := range flow.Steps {
			step := &flow.Steps[si]
			for _, name := range step.FieldNames() {
				for _, v := range mutate.Apply(step.Fields[name].Type, step.Fields[name].Seed) {
					cases = append(cases, testCase{flow: flow, stepIdx: si, target: "field:" + name,
						mutation: v.Label, fieldName: name, fieldValue: v.Value})
				}
			}
			rendered := step.Render(nil, m.Terminator)
			for _, v := range mutate.Frame(rendered) {
				cases = append(cases, testCase{flow: flow, stepIdx: si, target: "frame",
					mutation: v.Label, rawOverride: v.Value})
			}
		}
	}
	return cases
}
