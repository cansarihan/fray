# Contributing

Fray is a single Go binary with no dependencies outside the standard library.

## Adding a mutator

Mutators live in `internal/mutate`. Implement the interface, register it in an
`init`, and give it a `kind` so the engine applies it to the right fields
(`field`, `int`) or to whole frames. Add a test showing the variants it produces
and confirming they do not include the untouched seed.

## Adding to the demo

The demo target in `internal/target` carries deliberate bugs that `testdata/kv.json`
drives Fray to find. If you add a bug, add the step that reaches it and keep the
baseline flows clean — `internal/engine/engine_test.go` asserts the baseline
passes and the known bugs are found.

## Before a change

- `make lint` and `make test` must pass. Tests open loopback sockets.
- `gofmt` is enforced; run `make fmt`.

## Commits

Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `build:`, `ci:`), one
logical change each.
