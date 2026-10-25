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
- `cmd.list` — lists visible GUI-backed commands with their exact positive
  contract versions.
- `cmd.execute` — requires the `command` grant and an exact `{name, version}`.
  The runtime returns an accepted operation receipt, schedules the shared
  GUI command dispatcher at the next frame activation boundary, and sends
  `op.completed` only after the activation frame is ready and all command-owned
  job continuations have committed in a later ready frame. Their commits are
  fenced by the owning session generation. A disconnect cancels the operation
  and prevents any uncommitted continuation from mutating app state; already
  applied callback effects are not rolled back.
- `time.mode` — requires the `time` grant. `{"mode":"virtual"}` acquires
  the session's time lease and completes after its single zero-delta
  transition frame is ready. `{"mode":"wall"}` releases that lease after
  its transition frame. Repeating the current mode is idempotent and creates
  no frame. A driver transition during a held gesture is rejected.
- `time.advance` — requires the `time` grant and the controlling time lease.
  `{"duration_ns":"16666667"}` advances engine time by the bounded integer
  duration and completes after exactly one frame has a ready surface. Reads
  while Virtual is parked create no frame. Pure jobs in the prior cut's
  frontier finish before an authorized Virtual frame begins; their results
  commit in submission order during phase 1.
- `frame.run` — requires `time` plus the union of grants used by its plan.
	It accepts GUI-backed `cmd.execute` steps and the registered `input.*`
	steps, an explicit bounded frame tail, and registered read-only `then`
	requests. Input macros expand into their canonical per-frame atoms. A
	plan-local `bind_gesture` on `input.press` and matching `use_gesture` on
	`input.move`/`input.release` retain capture only within that plan; every
	bound gesture must be released before success. Nested plans, mode changes,
	and operation-control requests are rejected. A plan completes only after
	its final submitted frame is ready.
- `op.status` — reports the retained state and outcome of an operation owned
  by the authenticated session; expired outcomes return the operation.expired
  error.
- `input.click`, `input.press`, `input.release`, `input.move`, `input.drag`,
  `input.type`, `input.scroll`, and `input.focus` — require the `input` grant
  and enter the ordinary platform event queue. Each synthesized atom is
  collected at a frame boundary; Virtual mode remains parked until the time
  owner explicitly advances it. Device defaults to the current input modality
  (`mouse` unless touch is active) and accepts explicit `mouse` or `touch`.
  Mouse click is move/press/release; touch click is touch-down/touch-up. Drag
  uses `from` and `to` location oneofs plus `move_count` (default 5, range
  1..128), then emits its canonical start atom, interpolated moves, and
  release. Point methods accept `window:"primary"`; unavailable windows fail
  explicitly. Results report the selected device and one atom per frame.
  `input.press` returns a session- and connection-generation-bound
  `gesture_handle`; matching `input.move` and `input.release` requests must
  present it. Human input or connection loss cancels the unstarted remainder
  and clears capture at the next safe boundary. `input.focus` performs real
  Tab/Shift-Tab traversal and reports the number of hops.

Reads, streams, and artifacts remain later slices. The advertised registry
is the authoritative list of methods implemented by this build.

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

- **Command automation**: configure the launcher with `read,command` grants,
  then use `lurpic drive commands`, `lurpic drive execute --name <name>
  --version <n>`, and `lurpic drive op-status --operation-id <id>`. Add
  `--session-file <path>` to reuse a local 0600 credential file across CLI
  invocations. Without that option each invocation creates a new session.
- **Token rotation**: restart the app. The token file is regenerated next
  to the socket on every launch; there is no rotation without restart.
- **Lost command completion**: `execute` resumes the session and retries the
  same request ID after a lost receipt, then queries `op.status` if the
  completion notification is lost. Pass `--session-file` to retain the 0600
  credential if the CLI process exits; reconnect with that file and query the
  known operation ID. Do not issue the command under a new ID until its old
  outcome is known. A new app instance is a new identity and old outcomes are
  unavailable (`instance.changed`).
- **Virtual time**: request time.mode with mode=virtual before advancing; only
  the owning session may submit `time.advance` or `frame.run`. Disconnect
  releases ownership independently of engine time. Use time.advance with a
  duration_ns value from 1 through 1000000000 to request one frame, or
  frame.run for a bounded, ordered command plan. An input-only client never
  advances frames implicitly.
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
