// Package report renders a run's findings for the terminal, as JSON, or as a
// Markdown digest for an issue.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cansarihan/fray/internal/engine"
)

var outcomeWord = map[string]string{"timeout": "hang", "closed": "reset", "mismatch": "mismatch", "error": "error"}

// JSON writes a value as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table writes the run header and one row per finding, worst first.
func Table(w io.Writer, r engine.Result) {
	fmt.Fprintf(w, "%s against %s\n%d cases in %s · %s\n\n",
		r.Model, r.Target, r.Cases, r.Elapsed, counts(r))
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "no findings: the target withstood every mutation")
		return
	}
	findings := append([]engine.Finding(nil), r.Findings...)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Severity > findings[j].Severity })

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "RESULT\tFLOW/STEP\tTARGET\tMUTATION\tREPLY")
	for _, f := range findings {
		fmt.Fprintf(tw, "%s\t%s/%s\t%s\t%s\t%s\n",
			outcomeWord[f.Outcome], f.Flow, f.Step, f.Target, f.Mutation, clip(f.Reply, 40))
	}
	_ = tw.Flush()
}

// Markdown writes a digest grouped by outcome, with a replay block for the
// worst finding of each group.
func Markdown(w io.Writer, r engine.Result) {
	fmt.Fprintf(w, "# Fray report\n\n`%s` against `%s` — %d cases, %s.\n\n", r.Model, r.Target, r.Cases, counts(r))
	byOutcome := map[string][]engine.Finding{}
	order := []string{}
	for _, f := range r.Findings {
		if _, ok := byOutcome[f.Outcome]; !ok {
			order = append(order, f.Outcome)
		}
		byOutcome[f.Outcome] = append(byOutcome[f.Outcome], f)
	}
	sort.Slice(order, func(i, j int) bool { return weight(order[i]) > weight(order[j]) })
	for _, o := range order {
		group := byOutcome[o]
		fmt.Fprintf(w, "## %s (%d)\n\n", strings.Title(outcomeWord[o]), len(group)) //nolint:staticcheck
		for _, f := range group {
			fmt.Fprintf(w, "- **%s/%s** · %s · `%s`\n", f.Flow, f.Step, f.Target, f.Mutation)
		}
		fmt.Fprintf(w, "\nReplay the first:\n\n```\n%s```\n\n", replay(group[0]))
	}
}

func replay(f engine.Finding) string {
	var b strings.Builder
	for _, line := range f.Repro {
		b.WriteString(quote(line))
		b.WriteByte('\n')
	}
	return b.String()
}

// quote shows control bytes so a repro line can be read and retyped.
func quote(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\r':
			b.WriteString("\\r")
		case '\n':
			b.WriteString("\\n")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func counts(r engine.Result) string {
	parts := []string{}
	for _, o := range []string{"timeout", "closed", "mismatch", "error"} {
		if n := r.Counts[o]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, outcomeWord[o]))
		}
	}
	if len(parts) == 0 {
		return "0 findings"
	}
	return strings.Join(parts, ", ")
}

func weight(o string) int {
	switch o {
	case "timeout":
		return 3
	case "closed":
		return 2
	case "mismatch":
		return 1
	default:
		return 0
	}
}

func clip(s string, n int) string {
	s = quote(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
