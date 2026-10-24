package runtime

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// commandIDPattern: registration-time name validation (Debt-3 upgrade).
// Command IDs are the LDP app-semantic mutation surface (cmd.execute), so
// they are validated, versioned names — not palette plumbing strings.
var commandIDPattern = regexp.MustCompile(`^[a-z0-9.]+$`)

// CommandSpec is the enumerable command descriptor (cmd.list output): the
// registry's public mutation surface, versioned by the registry version.
type CommandSpec struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
}

// CommandEntry describes one registered command that can be surfaced by command palette UIs.
type CommandEntry struct {
	ID       string
	Title    string
	Category string
	Shortcut string
	IconRef  string
	Keywords []string
	Disabled bool
	Hidden   bool

	Execute func()
}

// CommandRegistry stores a searchable command catalog.
type CommandRegistry struct {
	version store.VersionSource

	mu       sync.RWMutex
	entries  map[string]CommandEntry
	OnChange signal.Signal[signal.Unit]
}

// NewCommandRegistry constructs an empty command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		entries:  make(map[string]CommandEntry),
		OnChange: signal.NewSignal[signal.Unit]("CommandRegistry.OnChange"),
	}
}

// Register adds a command entry. Registration is authoring time and
// fail-loud (Debt-3 upgrade): an invalid ID (must match [a-z0-9.]+), an
// empty title, or a collision with a live ID panics — the palette UIs and
// the LDP cmd.execute surface both key on these names, so a silent replace
// or a malformed ID would corrupt the app's public mutation surface.
func (r *CommandRegistry) Register(entry CommandEntry) {
	syncutil.AssertRuntimeThread()
	if r == nil {
		return
	}
	entry.ID = strings.TrimSpace(entry.ID)
	entry.Title = strings.TrimSpace(entry.Title)
	entry.Category = strings.TrimSpace(entry.Category)
	entry.Shortcut = strings.TrimSpace(entry.Shortcut)
	entry.IconRef = strings.TrimSpace(entry.IconRef)
	if !commandIDPattern.MatchString(entry.ID) {
		panic("runtime: command ID " + strconv.Quote(entry.ID) + " is invalid — IDs must match [a-z0-9.]+ (the cmd.execute surface)")
	}
	if entry.Title == "" {
		panic("runtime: command " + entry.ID + " registered with an empty title")
	}
	for i := range entry.Keywords {
		entry.Keywords[i] = strings.TrimSpace(entry.Keywords[i])
	}
	r.mu.Lock()
	if r.entries == nil {
		r.entries = make(map[string]CommandEntry)
	}
	if _, exists := r.entries[entry.ID]; exists {
		r.mu.Unlock()
		panic("runtime: command ID " + entry.ID + " registered twice — the cmd.execute surface must be collision-free")
	}
	r.entries[entry.ID] = entry
	r.version.Increment()
	r.mu.Unlock()
	r.OnChange.Emit(signal.Fired)
}

// List returns the registry's enumerable command specs, sorted by ID — the
// cmd.list output (Debt-3: the registry is the app's public mutation
// surface, enumerable and versioned).
func (r *CommandRegistry) List() []CommandSpec {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CommandSpec, 0, len(r.entries))
	for _, entry := range r.entries {
		out = append(out, CommandSpec{
			ID:       entry.ID,
			Title:    entry.Title,
			Category: entry.Category,
			Disabled: entry.Disabled,
			Hidden:   entry.Hidden,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Unregister removes a command by ID.
func (r *CommandRegistry) Unregister(id string) {
	syncutil.AssertRuntimeThread()
	if r == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	r.mu.Lock()
	if _, ok := r.entries[id]; !ok {
		r.mu.Unlock()
		return
	}
	delete(r.entries, id)
	r.version.Increment()
	r.mu.Unlock()
	r.OnChange.Emit(signal.Fired)
}

// Clear removes all commands from the registry.
func (r *CommandRegistry) Clear() {
	syncutil.AssertRuntimeThread()
	if r == nil {
		return
	}
	r.mu.Lock()
	if len(r.entries) == 0 {
		r.mu.Unlock()
		return
	}
	r.entries = make(map[string]CommandEntry)
	r.version.Increment()
	r.mu.Unlock()
	r.OnChange.Emit(signal.Fired)
}

// Lookup returns the command associated with the given ID.
func (r *CommandRegistry) Lookup(id string) (CommandEntry, bool) {
	if r == nil {
		return CommandEntry{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CommandEntry{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	return entry, ok
}

// Snapshot returns all entries in a stable search order.
func (r *CommandRegistry) Snapshot() []CommandEntry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.entries) == 0 {
		return nil
	}
	out := make([]CommandEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Execute runs the registered command handler, if any, and reports whether one was invoked.
func (r *CommandRegistry) Execute(id string) bool {
	if r == nil {
		return false
	}
	entry, ok := r.Lookup(id)
	if !ok || entry.Disabled || entry.Execute == nil {
		return false
	}
	entry.Execute()
	return true
}

// Version returns the current registry version.
func (r *CommandRegistry) Version() store.Version {
	if r == nil {
		return 0
	}
	return r.version.Current()
}
