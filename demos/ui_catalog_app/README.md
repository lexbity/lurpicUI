# UI Catalog

A standalone application for browsing and inspecting the lurpicUI mark inventory.


## Features


## Building

```bash
cd /home/lex/Public/lurpicUI/demos/ui_catalog_app
go build -o ui_catalog .
```

## Running

```bash
./ui_catalog
# or with custom window size
./ui_catalog -width=1600 -height=900
```

## Testing

```bash
go test ./...
```

## Architecture

The application follows the repo's shared engineering constraints:

- Domain truth lives in stores (`store/`)
- Facets are projection boundaries (`ui/`)
- Runtime owns mutable state
- Layout, projection, input, signals, and rendering remain phase-separated

## Data Model

### CatalogEntry
Each mark type has an entry with:
- Stable logical ID (e.g., `basic.rect`, `uiinput.button`)
- Display name and family classification
- Construction class (primitive/composed/generated)
- Interactive, theme-sensitive, and layout-sensitive flags
- Coverage status (implemented/partial/placeholder/missing)
- Notes and variant/state lists

### Coverage Status
- `implemented`: Full preview support
- `partial`: Some variants/states missing
- `placeholder`: Visual placeholder only
- `missing`: Not yet implemented
- `theme-dependent`: Varies by theme
- `layout-dependent`: Varies by layout

## File Structure

```
ui_catalog_app/
├── main.go              # Application entry point
├── deps.go              # Package imports for facet registration
├── go.mod, go.sum       # Module definition
├── model/
│   ├── catalog.go       # CatalogEntry, Family, CoverageStatus
│   └── inventory.go     # Standard catalog with all entries
├── store/
│   ├── filter.go        # FilterState store and helpers
│   ├── selection.go     # Selected entry store
│   └── catalog.go       # CatalogInstance and derived stores
└── ui/
    ├── shell.go         # Layout constants and bounds calculation
    ├── root.go          # Root facet managing shell layout
    ├── header.go        # Header with metadata display
    ├── sidebar.go       # Family navigation and filters
    ├── content.go       # Main content viewport
    ├── inspector.go     # Selected entry inspector
    ├── footer.go        # Status and counts footer
    └── root_test.go     # Unit tests
```
