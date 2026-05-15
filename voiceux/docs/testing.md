# Voice UX Testing

Voice UX is covered by deterministic unit tests and a fake host service.

## Test coverage

- mark descriptor registration
- service interface conformance
- store subscription lifecycle
- meter and vowel-space projection snapshots
- preset browser filtering and selection
- FX chain reorder and parameter dispatch
- calibration flow step progression
- stream widget bindings

## Acceptance pattern

Use `voiceux/testkit.NewFakeVoiceService()` to drive store updates, attach facets, and verify that:

- facets invalidate when host stores change
- commands are recorded instead of reaching a real audio backend
- projections remain deterministic

