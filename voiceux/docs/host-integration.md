# Voice UX Host Integration

Voice UX depends on a host-provided `VoiceService`.

The host owns:

- store mutation
- audio backend lifecycle
- `voicedsp` pipeline orchestration
- persistence of presets, calibration, and device state

The Voice UX package owns:

- reusable marks
- store-backed facets
- deterministic projection helpers
- host command dispatch helpers

## Typical data flow

1. The host copies audio and device state into `VoiceStores`.
2. Facets subscribe to those stores with `signal.Subscriptions`.
3. Facets invalidate only the affected dirty regions.
4. User actions dispatch typed `VoiceCommand` values or generic actions.

## Safety rule

Voice UX must not instantiate a `voicedsp` pipeline directly. The host creates and owns the audio pipeline.

