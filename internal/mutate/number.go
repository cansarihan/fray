package mutate

func init() { register(intEdges{}) }

// intEdges sends the values that break integer parsing and arithmetic: the
// type boundaries, a non-number, a huge number and a negative where a count is
// expected.
type intEdges struct{}

func (intEdges) kind() string     { return KindInt }
func (intEdges) Name() string     { return "int-edges" }
func (intEdges) Describe() string { return "boundary, oversized, negative and non-numeric integers" }
func (intEdges) Variants(string) []Variant {
	return []Variant{
		{Label: "zero", Value: "0"},
		{Label: "negative", Value: "-1"},
		{Label: "int32 max + 1", Value: "2147483648"},
		{Label: "int64 min", Value: "-9223372036854775808"},
		{Label: "overflow", Value: "99999999999999999999999999"},
		{Label: "not a number", Value: "NaN"},
		{Label: "leading zeros", Value: "0000000013"},
		{Label: "whitespace", Value: " 10 "},
	}
}
