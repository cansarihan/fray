package mutate

func init() {
	register(empty{})
	register(oversize{})
	register(formatString{})
	register(injectionChars{})
	register(controlBytes{})
}

// empty sends nothing where a value is expected.
type empty struct{}

func (empty) kind() string     { return KindField }
func (empty) Name() string     { return "empty" }
func (empty) Describe() string { return "an empty value where one is expected" }
func (empty) Variants(string) []Variant {
	return []Variant{{Label: "empty", Value: ""}}
}

// oversize sends values far longer than any sane input, to find missing length
// limits and buffer assumptions.
type oversize struct{}

func (oversize) kind() string     { return KindField }
func (oversize) Name() string     { return "oversize" }
func (oversize) Describe() string { return "values thousands of bytes long" }
func (oversize) Variants(seed string) []Variant {
	return []Variant{
		{Label: "1 KiB", Value: repeat("A", 1024)},
		{Label: "64 KiB", Value: repeat("A", 64*1024)},
		{Label: "repeated seed", Value: repeat(seed, 2048)},
	}
}

// formatString sends printf and shell-style markers that a careless server
// might interpolate.
type formatString struct{}

func (formatString) kind() string { return KindField }
func (formatString) Name() string { return "format-string" }
func (formatString) Describe() string {
	return "printf and template markers that must be treated as data"
}
func (formatString) Variants(string) []Variant {
	return []Variant{
		{Label: "printf", Value: "%s%s%s%s%n"},
		{Label: "template", Value: "${{7*7}}"},
		{Label: "brace", Value: "{0}{1}{2}"},
	}
}

// injectionChars sends the metacharacters of neighbouring languages so a value
// that leaks into SQL, a shell or the protocol framing shows itself.
type injectionChars struct{}

func (injectionChars) kind() string { return KindField }
func (injectionChars) Name() string { return "injection" }
func (injectionChars) Describe() string {
	return "quotes, separators and framing bytes that should stay literal"
}
func (injectionChars) Variants(seed string) []Variant {
	return []Variant{
		{Label: "quote", Value: seed + "'\""},
		{Label: "separator", Value: seed + " \t extra args"},
		{Label: "embedded newline", Value: seed + "\r\nINJECTED x"},
		{Label: "null byte", Value: seed + "\x00hidden"},
	}
}

// controlBytes sends bytes outside the printable range.
type controlBytes struct{}

func (controlBytes) kind() string     { return KindField }
func (controlBytes) Name() string     { return "control-bytes" }
func (controlBytes) Describe() string { return "non-printable and high bytes in a text field" }
func (controlBytes) Variants(string) []Variant {
	return []Variant{
		{Label: "control range", Value: "\x01\x02\x03\x1b\x7f"},
		{Label: "high bytes", Value: "\xff\xfe\x80\x81"},
		{Label: "invalid utf-8", Value: "\xc3\x28\xa0\xa1"},
	}
}
