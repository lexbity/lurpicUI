# LDP — Lurpic Drive Protocol (major 2)

LDP is the local automation seam: typed observations and GUI-backed actions
between the lurpicUI runtime and external drivers over an authenticated
Unix-domain socket. Protocol major **2** is a deliberate hard break: there
are no v1 adapters, aliases, or compatibility dispatch. The advertised
method set is the registry's *advertised* surface — a major-2 build never
puts an unported method on the wire. `TestDocsMatchRegistry` fails when
this document and the registry drift.

## Enabling

LDP is off by default (no socket, no token, no listener, no goroutines).
Enable per run:

```
LURPIC_DRIVE=on               # $XDG_RUNTIME_DIR/lurpic-<pid>/drive.sock, read grant
LURPIC_DRIVE=on:read,input    # explicit grants
LURPIC_DRIVE=unix:/abs/private/d.sock:read,input  # explicit absolute path
```

Unknown grants — including the v1 `write` spelling — fail startup.

## Sessions and requests

Each app launch creates a random instance identity. An authenticated
session owns monotonically increasing request IDs (decimal strings on the
wire), a bounded outcome ledger, and a connection generation. A session
survives transport loss for ten minutes of wall time; resume proves the
resume credential, increments the generation, and fences the replaced
generation's outstanding work — it never resumes execution automatically.
Request IDs are admitted contiguously: a retried retained ID replays its
original outcome, a changed payload is `request.conflict`, and an expired
ID is `request.expired` — it can never execute again. There is no direct
store mutation anywhere in the protocol: application actuation is GUI-backed
`input.*` and `cmd.execute` only.

## Grants

A capability set baked in by whoever launches the app and immutable per
session (resume never widens it): `read`, `input`, `command`, `time`,
`admin`. `command` is the application-command grant; `admin` authorizes
protocol administration only, never implicit input/command/time authority.

## Advertised methods (major 2)

The full table (including trusted in-process seam methods whose native v2
slices land in later slices) lives in `drive/proto/registry.go`. The
advertised wire surface today:

- `sys.hello` — version/instance/session negotiation; create with the
  launcher token, or resume with the instance/session/credential triple.
  The result carries instance/session/generation, `next_id`, grants, the
  schema digest, and the limits document.
- `sys.ping` — cut-exact liveness: the pong identifies the sealed cut and
  last submitted frame it observes.

Later slices port the remaining methods (time control, input, commands,
reads, streams, artifacts) with their complete contracts; until then the
server answers `protocol.unknown_method` for them and old clients are
rejected by the major handshake.

## Wire

UTF-8 NDJSON, one object per line, 4 MiB hard bound. Identifiers that
carry uint64 precision (request IDs, cuts, frames) are decimal strings.
Decoding is strict: duplicate JSON keys, unknown envelope fields, wrong
major, non-canonical IDs, non-object params, nesting beyond 32,
non-finite/overflowing numbers, and trailing data are rejected
before admission with typed errors (`protocol.malformed`,
`protocol.line_limit`, `protocol.version_mismatch`) that never consume a
request ID.

## Security

The socket is the trust boundary: a private (0700), UID-owned parent
directory; socket and token inodes 0600; exclusive no-follow token
creation; a pre-existing socket or token path is a startup failure (stale
paths are cleaned by an operator, never unlinked blind); token comparison
is constant-time. Android: LDP compiles only in debuggable builds; the
socket never leaves the device (`adb forward` for remote debugging); MCP
exposure is the bridge author's decision (see `lurpic mcp init`).

## Ops runbook

- **Token rotation**: restart the app. The token file is regenerated next
  to the socket on every launch; there is no rotation without restart.
- **Lost responses**: resume the session with its credential and query the
  retained outcome by request ID. Never replay a pending mutation under a
  fresh identity; a new app instance is a new instance identity and old
  outcomes are unavailable (`instance.changed`).
- **Request/result expiry**: retention is bounded (entries and bytes per
  session); an evicted ID answers `request.expired` and the high-water
  mark never moves backwards.
- **Preemption expectations**: any human (platform) event severs an
  in-flight `input.*` step mid-gesture; ownership flips to human and the
  verb replies `input.preempted`. Drivers must handle it rather than retry
  blindly. Disconnect fences a lost generation's future work at the next
  safe boundary; applied effects stand.
- **Budget interpretation**: drive commands admit at the frame cut on the
  runtime thread; a frame carrying commands pays their handler cost inside
  that frame's budget. The E5 drive row shows per-frame drained counts.
- **Android**: the drive seam compiles only into debuggable builds (the
  `lurpicdrive` build tag, set by `lurpic build android` for debug APKs).
  Release builds refuse `LURPIC_DRIVE` at startup with "drive unavailable".
  Forward the socket with `adb forward` for remote `lurpic drive` use.
