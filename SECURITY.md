# Security policy

Fray sends malformed input to a network service, so it must only be aimed at a
target you are authorized to test. The target defaults to loopback; a
non-loopback target is refused unless `-allow-remote` is passed. The operator
panel starts a campaign against that one fixed target and cannot be redirected
by a request.

Report a suspected vulnerability privately through GitHub's security advisory
flow for this repository rather than a public issue. Include a minimal
reproduction and the affected version.
