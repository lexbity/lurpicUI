# Voice UX Marks and Facets

The `voiceux` package provides reusable UI contracts for voice-driven surfaces.

## Marks

Voice UX mark types are local to the package and registered through `voiceux/marks`.

Required marks:

- `voice_device_selector`
- `voice_meter`
- `voice_preset_browser`
- `voice_fx_chain`
- `voice_calibration_flow`
- `voice_vowel_space`
- `voice_mixer_strip`
- `voice_stream_widget`

These marks are intentionally small descriptors. They do not own audio processing and do not reach into `voicedsp` internals.

## Facets

Voice UX facets subscribe to host-owned stores and dispatch typed commands through `VoiceService`.

Facet responsibilities:

- `DeviceSelectorFacet`: device selection and refresh dispatch
- `MeterFacet`: live level and confidence visualization
- `PresetBrowserFacet`: preset filtering and preset selection
- `FXChainFacet`: effect chain viewing, bypass, and parameter dispatch
- `CalibrationFlowFacet`: calibration prompt flow and progress display
- `VowelSpaceFacet`: F1/F2 projection and calibration visualization
- `MixerStripFacet`: bus gain and monitor routing controls
- `StreamWidgetFacet`: compact live voice controls for panel embedding

