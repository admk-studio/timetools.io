# timetools

**The time service you can curl.**

```
$ curl timetools.io/tokyo

  Tokyo — Asia/Tokyo

  18:52:04  Tuesday, 14 July 2026
  UTC+09:00 · JST · no seasonal clock changes

  sunrise 04:35 · sunset 18:57 · daylight 14h22m

  UTC 09:52 · unix 1784022724
```

No signup, no API key, no JavaScript required. One URL gives you the
current time, UTC offset, DST status, the next clock change, and sunrise
and sunset — for any city, in your terminal, as JSON, or in a browser.

[![CI](https://github.com/admk-studio/timetools.io/actions/workflows/ci.yml/badge.svg)](https://github.com/admk-studio/timetools.io/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/admk-studio/timetools.io)](https://goreportcard.com/report/github.com/admk-studio/timetools.io)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Zero dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen)

## Compare cities

Ask for several places at once and you get a comparison table plus a
working-hours chart — the fastest way to find a meeting slot across
time zones:

```
$ curl timetools.io/nyc/london/tokyo

  New York   05:53     Tue, Jul 14     UTC-04:00
  London     10:53     Tue, Jul 14     UTC+01:00   +5h
  Tokyo      18:53     Tue, Jul 14     UTC+09:00   +13h

  working hours, on New York's clock
             0  3  6  9  12 15 18 21
  New York   ·····┃···█████████······
  London     ····█┃███████···········
  Tokyo      █████┃··············████

  no shared working hours
```

The `┃` column is right now; the blocks are 09:00–18:00 in each city's
local day. The chart samples each labeled hour; a `-` marks an hour
skipped by DST, and repeated hours appear once. The overlap below it uses
exact minute boundaries. Drop Tokyo and it tells you `everyone is at work
09:00-13:00, New York time`.

## What it understands

| You type | You get |
| --- | --- |
| `/tokyo`, `/new_york`, `/são_paulo` | city names, spacing and accents forgiven |
| `/nyc`, `/sf`, `/kl` | common abbreviations |
| `/america/new_york` | any IANA zone name, any capitalization |
| `/pst`, `/ist`, `/cet` | time zone abbreviations (mapped to their most common meaning) |
| `/utc+5:30`, `/gmt-3` | fixed UTC offsets |
| `/miami`, `/geneva`, `/osaka` | cities that share a zone but not a sunrise — coordinates are per-city |
| `/unix` | `1784022724` — epoch seconds, nothing else |
| `/utc` | `2026-07-14T09:52:20Z` — nothing else |
| `/zones?q=india` | search the zone list |

Typos get suggestions (`/tokio` → *did you mean tokyo?*), and everything
above also works in a browser — the same URLs render as live-updating
pages with a ticking clock.

## JSON for scripts

Append `?format=json` or send `Accept: application/json`:

```
$ curl -s timetools.io/berlin?format=json
{
  "generated": "2026-07-14T09:52:20Z",
  "locations": [
    {
      "name": "Berlin",
      "timezone": "Europe/Berlin",
      "time": "2026-07-14T11:52:20+02:00",
      "unix": 1784022740,
      "utc_offset": "+02:00",
      "abbreviation": "CEST",
      "dst": true,
      "sun": {
        "sunrise": "2026-07-14T05:00:11+02:00",
        "sunset": "2026-07-14T21:24:07+02:00",
        "daylight_minutes": 983
      },
      "next_change": {
        "at": "2026-10-25T02:00:00+01:00",
        "from": "+02:00",
        "to": "+01:00"
      }
    }
  ]
}
```

The JSON shape is a stable contract: fields get added, never renamed or
removed. CORS is open, so you can fetch it straight from a web page.

### Options

| Query | Effect |
| --- | --- |
| `?12` | 12-hour clock |
| `?plain` | no colors, pure ASCII — for pipes and logs |
| `?format=json` / `?format=html` / `?format=text` | force a format |

### Handy aliases

```sh
alias now='curl -s timetools.io'
alias when='curl -s timetools.io/nyc/london/tokyo'
```

## Self-hosting

The per-client limiter stores at most 8,192 identities. At capacity, new
identities receive HTTP 429 until an idle bucket can be safely replaced;
existing clients retain their limits and health checks remain available.

The whole service is one static binary with **zero dependencies** — the
time zone database and the sunrise math are compiled in. The container
image is built `FROM scratch` and weighs a few megabytes.

```sh
git clone https://github.com/admk-studio/timetools.io && cd timetools.io
docker compose up -d
# or, without Docker:
make build && ./bin/timetools
```

Works out of the box on Coolify, CapRover, Fly.io, or a bare VPS: point
the platform at this repo, it builds the Dockerfile, and `/health` is
your health check. Behind a reverse proxy set `TT_TRUST_PROXY=true` and
`TT_TRUSTED_PROXIES` to its IP addresses or CIDRs (for example,
`127.0.0.1,::1` for a proxy on the same host). Startup fails if proxy
trust is enabled without this list. Use the actual proxy addresses as
seen by the server, including container network addresses where applicable.
Only listed peers may supply forwarded headers. The proxy must overwrite
or append the connecting client's IP to `X-Forwarded-For` and overwrite
`X-Forwarded-Proto`. The server walks the IP chain from right to left,
stopping at the first untrusted address. Keep trusted ranges limited to
networks controlled by your proxies.

### Configuration

Everything is optional; defaults give a working server.

| Variable | Default | Purpose |
| --- | --- | --- |
| `TT_ADDR` | `:8080` | listen address |
| `TT_BASE_URL` | `timetools.io` | hostname printed in examples and hints |
| `TT_RATE_RPM` | `120` | sustained requests/minute per client |
| `TT_RATE_BURST` | `30` | extra burst allowance |
| `TT_TRUST_PROXY` | `false` | enable forwarded headers from configured proxies |
| `TT_TRUSTED_PROXIES` | *(empty)* | comma-separated proxy IPs/CIDRs; required when proxy trust is enabled |
| `TT_REPO_URL` | this repo | source link in the footer |
| `TT_LINK_TEXT` / `TT_LINK_URL` | *(empty)* | optional footer link for your instance |

## How it works

- **Time zones** — the canonical zone list is generated from IANA's
  `zone1970.tab` and compiled in, with a curated alias table for the
  names people actually type. Zone data itself comes embedded via Go's
  `time/tzdata`, so the binary is self-contained even in an empty
  container.
- **Next clock change** — Go doesn't expose the zone transition table,
  so the server probes forward and binary-searches the exact second the
  offset changes. That's how it can say *clocks go back 1h on Sun,
  Oct 25 at 03:00*.
- **Sunrise/sunset** — the NOAA sunrise equation, implemented in ~80
  lines. Aliased cities carry their own coordinates: Miami and New York
  share a zone, but not a sunrise.
- **Terminal vs browser** — content negotiation looks at `?format=`,
  then `Accept`, then the user agent. Unknown clients get text, because
  mangled text in a browser beats HTML soup in a terminal.

Standard library only. No frameworks, no third-party packages, no
`go.sum` at all.

## Development

```sh
make run    # local server on :8080
make test
make fmt vet
```

See [CONTRIBUTING.md](CONTRIBUTING.md) — adding your city is a
three-line pull request.

## License

[MIT](LICENSE). Run your own, fork it, embed it — enjoy.

---

The public instance runs at **[timetools.io](https://timetools.io)**,
built and hosted by the team behind
[todaydateandtime.com](https://todaydateandtime.com).
