package engine

import (
	"context"

	"github.com/cansarihan/fray/internal/conn"
	"github.com/cansarihan/fray/internal/model"
)

// severity ranks outcomes: a hang or reset mid-protocol is worse than a reply
// that merely fails to match.
var severity = map[conn.Outcome]int{conn.Timeout: 3, conn.Closed: 2, conn.Mismatch: 1}

// baseline runs every flow unmutated and reports whether they all complete.
func baseline(ctx context.Context, m *model.Model, opts Options) (bool, error) {
	for fi := range m.Flows {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		session, _, err := conn.Dial(opts.Network, opts.Addr, m, opts.Timeout)
		if err != nil {
			return false, err
		}
		ok := true
		for si := range m.Flows[fi].Steps {
			step := &m.Flows[fi].Steps[si]
			if r := session.Send(step.Render(nil, m.Terminator), step); r.Outcome != conn.OK {
				ok = false
				break
			}
		}
		session.Close()
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// runCase drives one flow up to the target step, injects the mutation there,
// and reports a finding when the turn ends in anything but a clean reply.
func runCase(m *model.Model, opts Options, tc testCase) *Finding {
	session, _, err := conn.Dial(opts.Network, opts.Addr, m, opts.Timeout)
	if err != nil {
		// The target is unreachable before this case even runs — a reset the
		// previous case may have caused. Report it against this case.
		return &Finding{Flow: tc.flow.Name, Step: tc.flow.Steps[tc.stepIdx].Name, Target: tc.target,
			Mutation: tc.mutation, Outcome: string(conn.Closed), Severity: severity[conn.Closed],
			Repro: nil, Reply: dialError(err)}
	}
	defer session.Close()

	var repro []string
	// Replay the steps before the target one with seed values.
	for si := 0; si < tc.stepIdx; si++ {
		step := &tc.flow.Steps[si]
		raw := step.Render(nil, m.Terminator)
		repro = append(repro, raw)
		if r := session.Send(raw, step); r.Outcome != conn.OK {
			// The run fell over before reaching the mutation; not this case's fault.
			return nil
		}
	}

	step := &tc.flow.Steps[tc.stepIdx]
	raw := tc.rawOverride
	if raw == "" {
		values := map[string]string{}
		if tc.fieldName != "" {
			values[tc.fieldName] = tc.fieldValue
		}
		raw = step.Render(values, m.Terminator)
	}
	repro = append(repro, raw)

	reply := session.Send(raw, step)
	if reply.Outcome == conn.OK {
		return nil
	}
	return &Finding{
		Flow: tc.flow.Name, Step: step.Name, Target: tc.target, Mutation: tc.mutation,
		Outcome: string(reply.Outcome), Sent: clip(raw, 160), Reply: clip(reply.Line, 160),
		Repro: repro, Severity: severity[reply.Outcome],
	}
}

func dialError(err error) string {
	if err == nil {
		return ""
	}
	return clip(err.Error(), 160)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
