// Package model describes a text protocol the way Fray needs to exercise it:
// a set of flows, each a sequence of steps, each step a message template with
// typed fields and a pattern the reply must match. The same description drives
// the baseline run, the mutations and the state graph in the panel.
package model

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
)

// Field is one variable part of a message.
type Field struct {
	Type string `json:"type"` // "string", "int" or "token"
	Seed string `json:"seed"` // value used in the baseline run
}

// Step is one message sent and the reply expected.
type Step struct {
	Name   string           `json:"name"`
	Send   string           `json:"send"`   // template; {field} is replaced
	Expect string           `json:"expect"` // regular expression the reply must match
	Fields map[string]Field `json:"fields,omitempty"`

	expect *regexp.Regexp
}

// Flow is an ordered conversation, such as log in, store, read, quit.
type Flow struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

// Model is a whole protocol.
type Model struct {
	Name       string `json:"name"`
	Banner     string `json:"banner,omitempty"` // pattern the greeting must match
	Terminator string `json:"terminator"`       // appended to every message, e.g. "\r\n"
	Flows      []Flow `json:"flows"`

	banner *regexp.Regexp
}

var placeholder = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// Load reads and validates a model file.
func Load(path string) (*Model, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied model file
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse validates a model: every pattern compiles, every placeholder names a
// declared field, every field has a known type.
func Parse(data []byte) (*Model, error) {
	var m Model
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	if len(m.Flows) == 0 {
		return nil, fmt.Errorf("model %q has no flows", m.Name)
	}
	if m.Terminator == "" {
		m.Terminator = "\r\n"
	}
	if m.Banner != "" {
		re, err := regexp.Compile(m.Banner)
		if err != nil {
			return nil, fmt.Errorf("banner: %w", err)
		}
		m.banner = re
	}
	for fi := range m.Flows {
		flow := &m.Flows[fi]
		if len(flow.Steps) == 0 {
			return nil, fmt.Errorf("flow %q has no steps", flow.Name)
		}
		for si := range flow.Steps {
			step := &flow.Steps[si]
			re, err := regexp.Compile(step.Expect)
			if err != nil {
				return nil, fmt.Errorf("flow %q step %q: %w", flow.Name, step.Name, err)
			}
			step.expect = re
			for _, match := range placeholder.FindAllStringSubmatch(step.Send, -1) {
				if _, ok := step.Fields[match[1]]; !ok {
					return nil, fmt.Errorf("flow %q step %q uses undeclared field {%s}", flow.Name, step.Name, match[1])
				}
			}
			for name, f := range step.Fields {
				switch f.Type {
				case "string", "int", "token":
				default:
					return nil, fmt.Errorf("flow %q step %q field %q: unknown type %q", flow.Name, step.Name, name, f.Type)
				}
			}
		}
	}
	return &m, nil
}

// BannerOK reports whether a greeting matches the model's banner pattern.
func (m *Model) BannerOK(greeting string) bool {
	return m.banner == nil || m.banner.MatchString(greeting)
}

// Matches reports whether a reply is the one the step expects.
func (s *Step) Matches(reply string) bool { return s.expect.MatchString(reply) }

// Render fills a step's template with values, falling back to each field's
// seed, and appends the terminator.
func (s *Step) Render(values map[string]string, terminator string) string {
	out := placeholder.ReplaceAllStringFunc(s.Send, func(p string) string {
		name := p[1 : len(p)-1]
		if v, ok := values[name]; ok {
			return v
		}
		return s.Fields[name].Seed
	})
	return out + terminator
}

// FieldNames returns a step's fields in a stable order.
func (s *Step) FieldNames() []string {
	names := make([]string, 0, len(s.Fields))
	for n := range s.Fields {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Edge is a transition between two named steps in some flow.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph returns every step name and the transitions the flows take between
// them, starting from a virtual "connect" node.
func (m *Model) Graph() (nodes []string, edges []Edge) {
	seen := map[string]bool{"connect": true}
	nodes = append(nodes, "connect")
	edgeSeen := map[string]bool{}
	for _, f := range m.Flows {
		prev := "connect"
		for _, s := range f.Steps {
			if !seen[s.Name] {
				seen[s.Name] = true
				nodes = append(nodes, s.Name)
			}
			key := prev + "\x00" + s.Name
			if !edgeSeen[key] {
				edgeSeen[key] = true
				edges = append(edges, Edge{From: prev, To: s.Name})
			}
			prev = s.Name
		}
	}
	return nodes, edges
}

// String is a one-line description.
func (m *Model) String() string {
	steps := 0
	for _, f := range m.Flows {
		steps += len(f.Steps)
	}
	return fmt.Sprintf("%s: %d flows, %d steps", m.Name, len(m.Flows), steps)
}
