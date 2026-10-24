# LDP — Lurpic Drive Protocol

LDP is the local automation seam: a versioned, typed command bus between the
lurpicUI runtime and external drivers over a Unix-domain socket. Every verb
has a GUI twin proven equivalent by the differential corpus
(`input/equivalence`). This document is generated-checked against the
protocol registry — `TestDocsMatchRegistry` fails when the two drift.

## Enabling

LDP is off by default (no socket, no token, no listener). Enable per run:

```
LURPIC_DRIVE=on               # $XDG_RUNTIME_DIR/lurpic-<pid>/drive.sock, read grant
LURPIC_DRIVE=on:read,input    # explicit grants
LURPIC_DRIVE=unix:/tmp/d.sock:read,input  # explicit path
```

## Grants

A capability set baked in by whoever launches the app: `read`, `input`,
`write`, `time`, `admin`. Method dispatch checks the grant before execution;
denials are typed errors and increment a counter.

## Methods

The full method table lives in `drive/proto/registry.go` (grant, mutating
flag, tier, equivalence twin). `sys.info` reports the live list; `lurpic
drive --check` compares digests client-vs-server.

The registry's methods:

- `sys.hello`, `sys.ping`, `sys.info` — handshake and liveness.
- `time.mode`, `time.advance` — owned time (virtual mode steps exactly one
  frame per advance).
- `frame.run` — a frame transaction: steps apply one per cut, `advance`
  policies with a mandatory `max`, `then` reads at the final cut.
- `locate.resolve`, `tree.dump`, `facet.query` — identity and structure
  (author TestIDs, accessibility role/name, locator grammar, arranged
  bounds, opt-in `DriveDetails`).
- `store.list`, `store.read`, `derived.read` — domain reads (named and
  structural addresses; derived reads peek, never recompute).
- `store.write` — the debugging write of last resort (labeled
  `bypassed_app_invariants`; collections accept insert/update/remove).
- `cmd.list`, `cmd.execute` — the app's public command surface (versioned,
  collision-free names).
- `input.click`, `input.press`, `input.release`, `input.move`, `input.drag`,
  `input.type`, `input.scroll`, `input.focus` — input synthesis through the
  real platform queue; each has a differential-equivalence GUI twin.
- `frame.pixels`, `frame.packet` — render reads (software surface PNG;
  packet digest + epoch).
- `sub.watch`, `sub.cancel` — subscriptions (see below).

## Subscriptions

`sub.watch` streams events over the same connection: each event
carries the frame epoch it refers to. Per-subscription FIFO depth is 256; a
reader that stalls past 5 seconds is detached with
`sub.dead` (reason: backpressure) — events are never silently dropped.

## Security

The socket is the trust boundary: filesystem mode 0600, token file 0600
next to it. Android: LDP compiles only in debuggable builds. The socket
never leaves the device (`adb forward` for remote debugging); MCP exposure
is the bridge author's decision (see `lurpic mcp init`).

## Ops runbook

- **Token rotation**: restart the app. The token file is regenerated next to
  the socket on every launch; there is no rotation without restart.
- **Detach diagnosis**: `DriveBackpressureDetached` spikes (see the E5 drive
  row / `sys.info`) mean a subscriber stopped draining; the connection is
  gone — reconnect and re-`sub.watch`. Events are never replayed.
- **Preemption expectations**: any human (platform) event severs an in-flight
  `input.*` step mid-gesture. The verb replies `input.preempted`, ownership
  flips to human, and `own.preempted` fans out to subscribers. Drivers must
  handle it rather than retry blindly.
- **Budget interpretation**: drive commands drain at the frame cut; a frame
  carrying commands pays their handler cost inside that frame's budget. The
  E5 drive row shows per-frame drained counts — sustained non-zero values
  mean a driver is commanding every frame.
- **Android**: the drive seam compiles only into debuggable builds (the
  `lurpicdrive` build tag, set by `lurpic build android` for debug APKs).
  Release builds refuse `LURPIC_DRIVE` at startup with "drive unavailable".
  Forward the socket with `adb forward` for remote `lurpic drive` use.
