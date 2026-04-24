# UI Replay Scenario Schema

## Version 1.0

This document describes the JSON schema for replay scenarios.

## Schema Structure

```json
{
  "id": "unique.scenario.identifier",
  "display_name": "Human Readable Name",
  "schema": "1.0",
  "description": "Optional scenario description",
  "required_scene": "scene_name",
  "environment": {
    "theme": "baseline",
    "density": "default",
    "backend": "software",
    "window_size": {
      "width": 1400,
      "height": 900
    }
  },
  "capabilities": ["scene_load", "screenshots"],
  "expected_state": {
    "scene_id": "expected_scene",
    "theme": "baseline",
    "density": "default",
    "focus_target": "control_id"
  },
  "actions": [...],
  "assertions": [...],
  "artifacts": [...],
  "tags": ["tag1", "tag2"]
}
```

## Fields

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique scenario identifier (dot-notation recommended) |
| `display_name` | string | Human-readable name for UI display |
| `schema` | string | Schema version (must be "1.0") |
| `actions` | array | List of actions to execute |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Detailed scenario description |
| `required_scene` | string | Scene required for this scenario |
| `environment` | object | Environment configuration |
| `capabilities` | array | Required runtime capabilities |
| `expected_state` | object | Expected initial state |
| `assertions` | array | State assertions to validate |
| `artifacts` | array | Expected output artifacts |
| `tags` | array | Classification tags |

`display_name` and `schema` are required for registry-loaded scenarios. Tests that only need a lightweight fixture should use `model.NewFixtureScenario(...)` so the runtime contract stays explicit.

## Actions

### Action Structure

```json
{
  "type": "action_type",
  "target": {
    "logical_id": "control.id",
    "x": 100,
    "y": 200,
    "fallback": {
      "logical_id": "fallback.id"
    }
  },
  "params": {
    "key": "value"
  }
}
```

### Action Types

| Type | Description | Required Params |
|------|-------------|---------------|
| `scene_load` | Load a scene | `scene` |
| `click` | Click/tap a target | target with `logical_id` or coordinates |
| `pointer_move` | Move pointer | `x`, `y` |
| `drag` | Drag target | target, `dest_x`, `dest_y` |
| `key_input` | Send key press | `key` |
| `text_input` | Type text | `text` |
| `ime_hook` | IME action | `action` |
| `wait_frames` | Wait N frames | `frames` |
| `wait_idle` | Wait for idle | optional `timeout_ms` |
| `switch_theme` | Change theme | `theme` |
| `switch_density` | Change density | `density` |
| `resize_window` | Resize window | `width`, `height` |
| `assert_state` | Assert state | assertion params |
| `screenshot` | Capture screenshot | `name` |
| `export_bundle` | Export artifact bundle | `path` |

### Target Resolution

Targets support stable logical IDs with coordinate fallback:

1. Primary target is checked first (logical ID or coordinates)
2. If primary is empty and fallback exists, fallback is used
3. Coordinates are only used when logical ID resolution fails

## Assertions

### Assertion Structure

```json
{
  "type": "assertion_type",
  "params": {
    "expected": "value"
  }
}
```

### Assertion Types

| Type | Description | Params |
|------|-------------|--------|
| `scene_id` | Assert current scene | `expected` |
| `control_state` | Assert control state | `control_id`, `expected` |
| `theme_state` | Assert active theme | `expected` |
| `density_state` | Assert density | `expected` |
| `focus_owner` | Assert focus | `expected` |
| `event_present` | Assert event occurred | `event_type` |
| `store_summary` | Assert store state | `store_id`, `expected` |
| `signal_summary` | Assert signal state | `signal_id`, `expected` |
| `screenshot` | Assert screenshot exists | `name` |
| `diagnostics` | Assert diagnostics | `expected` |
| `frame_count` | Assert frame count | `min`, `max` |

## Artifacts

### Artifact Structure

```json
{
  "type": "artifact_type",
  "name": "artifact_name",
  "required": true
}
```

### Artifact Types

| Type | Description |
|------|-------------|
| `screenshot` | PNG screenshot |
| `log` | Execution log |
| `scene_export` | Scene state export |
| `diagnostics` | Diagnostics snapshot |

## Capabilities

Capabilities declare what the scenario needs from the runtime:

| Capability | Description |
|------------|-------------|
| `scene_load` | Scene loading |
| `theme_switch` | Theme switching |
| `density_switch` | Density switching |
| `pointer_input` | Pointer/touch input |
| `keyboard_input` | Keyboard input |
| `text_input` | Text entry |
| `ime` | IME support |
| `screenshots` | Screenshot capture |
| `assertions` | State assertions |
| `background_jobs` | Background job handling |

Capability declarations are validated against this list. Unknown capability strings, empty entries, and duplicates are rejected by the scenario validator.

## Validation

Scenarios are validated before execution:

1. **Required fields** - id, display_name, schema
2. **Schema version** - must be in supported versions list
3. **Action types** - all types must be supported
4. **Assertion types** - all types must be supported
5. **Artifact types** - all types must be supported
6. **Capability values** - all capabilities must be supported and unique
7. **Duplicate names** - artifact names must be unique

### Validation Errors

Errors are structured with field, message, and optional step:

```go
type ValidationError struct {
    Field   string // e.g., "action.type"
    Message string // e.g., "unsupported action type: 'invalid'"
    Step    int    // 1-based step index for action/assertion errors
}
```

## Example Scenario

```json
{
  "id": "basic.navigation_test",
  "display_name": "Basic Navigation Test",
  "schema": "1.0",
  "description": "Tests basic navigation controls",
  "required_scene": "basic",
  "environment": {
    "theme": "baseline",
    "density": "default",
    "backend": "software",
    "window_size": {
      "width": 800,
      "height": 600
    }
  },
  "capabilities": ["scene_load", "pointer_input", "screenshots", "assertions"],
  "expected_state": {
    "scene_id": "basic",
    "theme": "baseline",
    "density": "default"
  },
  "actions": [
    {
      "type": "scene_load",
      "params": {
        "scene": "basic"
      }
    },
    {
      "type": "wait_frames",
      "params": {
        "frames": 3
      }
    },
    {
      "type": "click",
      "target": {
        "logical_id": "nav.next",
        "fallback": {
          "x": 700,
          "y": 300
        }
      }
    },
    {
      "type": "screenshot",
      "params": {
        "name": "after_click"
      }
    }
  ],
  "assertions": [
    {
      "type": "scene_id",
      "params": {
        "expected": "basic"
      }
    }
  ],
  "artifacts": [
    {
      "type": "screenshot",
      "name": "after_click",
      "required": true
    }
  ],
  "tags": ["basic", "navigation", "smoke-test"]
}
```
