# Contributing

Thanks for wanting to help! This is a small codebase on purpose — the
whole service is standard library Go, and we'd like to keep it that way.

## Getting started

Use Go 1.24 or newer. The browser clock regression tests also need Node.js
18 or newer; production builds still require only Go.

```sh
git clone https://github.com/admk-studio/timetools.io
cd timetools.io
make run      # server on localhost:8080
make test
```

## Adding your city

The most common contribution: your city resolves to the wrong place or
not at all. City aliases live in `internal/tz/aliases.go`. Add an entry
with the IANA zone, the display name, and the city's coordinates
(they're used for sunrise/sunset, so please look them up rather than
copying a neighbour):

```go
"my city": city("Europe/Somewhere", "My City", 12.34, 56.78),
```

Then add a line to `TestResolve` in `internal/tz/tz_test.go` and run
`make test`.

## Pull requests

- `make fmt vet test` must pass.
- No new dependencies. Seriously — the zero-dependency build is a
  feature. If something seems impossible without a library, open an
  issue first and we'll figure it out.
- Keep the output format stable. Scripts parse it; the JSON shape is a
  public contract (add fields, never rename or remove).
- One change per PR, with a test that would fail without it.

## Regenerating the zone table

`internal/tz/zones_gen.go` is generated from IANA's `zone1970.tab`.
The complete name/link index in `internal/tz/names_gen.go` comes from the
Go toolchain's `zoneinfo.zip`. Regenerate with an updated Go toolchain and
host tzdata after a tzdata release:

```sh
make generate
make test
```
