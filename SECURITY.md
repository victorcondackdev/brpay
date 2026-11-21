# Security policy

## Supported versions

Only the latest release receives fixes while the module is at v0.x.

## Reporting a vulnerability

Please do not open a public issue. Use GitHub's private vulnerability
reporting ("Security" tab, "Report a vulnerability") with a description,
the affected version and, if possible, an input that reproduces the problem.
You can expect an acknowledgement within a few days.

In scope: panics, excessive CPU or memory use on crafted input, and any case
where an invalid code is accepted as valid (or a valid one rejected) by
`Decode`, `Parse` or `Validate`. Every parser in this module is fuzzed; a
fuzz corpus entry is the ideal reproducer.

The playground runs entirely in the browser and sends no data anywhere.
