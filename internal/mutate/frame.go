package mutate

func init() { register(framing{}) }

// framing attacks the message envelope rather than a field: it strips the
// terminator, doubles it, and smuggles a second command inside one line. These
// apply to a rendered message, not a single field.
type framing struct{}

func (framing) kind() string     { return KindFrame }
func (framing) Name() string     { return "framing" }
func (framing) Describe() string { return "a message with a broken, doubled or smuggled line terminator" }
func (framing) Variants(message string) []Variant {
	trimmed := message
	for len(trimmed) > 0 && (trimmed[len(trimmed)-1] == '\n' || trimmed[len(trimmed)-1] == '\r') {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return []Variant{
		{Label: "no terminator", Value: trimmed},
		{Label: "doubled terminator", Value: message + "\r\n"},
		{Label: "lone LF", Value: trimmed + "\n"},
		{Label: "smuggled command", Value: trimmed + "\r\nSMUGGLED command\r\n"},
		{Label: label("%d byte line", 70000), Value: repeat("A", 70000) + "\r\n"},
	}
}

// Frame returns the framing variants for a rendered message.
func Frame(message string) []Variant { return framing{}.Variants(message) }
