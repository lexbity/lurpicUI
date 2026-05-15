package testkit

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/voiceux"
)

// FakeVoiceService is a lightweight host-service double for tests.
type FakeVoiceService struct {
	mu       sync.Mutex
	stores   *voiceux.VoiceStores
	registry voiceux.StaticDescriptorRegistry

	Commands []voiceux.VoiceCommand
	Actions  []ActionRecord
	Err      error
}

// ActionRecord captures a dispatched generic action.
type ActionRecord struct {
	ID   string
	Args map[string]any
}

// NewFakeVoiceService constructs a test double with default stores and descriptors.
func NewFakeVoiceService() *FakeVoiceService {
	return &FakeVoiceService{
		stores:   voiceux.NewVoiceStores(),
		registry: voiceux.DefaultDescriptorRegistry(),
	}
}

// Stores returns the mutable voice store set.
func (f *FakeVoiceService) Stores() *voiceux.VoiceStores {
	if f == nil {
		return nil
	}
	return f.stores
}

// DispatchVoiceCommand records one typed command.
func (f *FakeVoiceService) DispatchVoiceCommand(cmd voiceux.VoiceCommand) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Commands = append(f.Commands, cmd)
	return f.Err
}

// DispatchAction records one generic action.
func (f *FakeVoiceService) DispatchAction(actionID string, args map[string]any) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cloned := make(map[string]any, len(args))
	for k, v := range args {
		cloned[k] = v
	}
	f.Actions = append(f.Actions, ActionRecord{ID: actionID, Args: cloned})
	return f.Err
}

// DescriptorRegistry returns the static descriptor bundle.
func (f *FakeVoiceService) DescriptorRegistry() voiceux.DescriptorRegistry {
	if f == nil {
		return voiceux.DefaultDescriptorRegistry()
	}
	return f.registry
}

// LastCommand returns the most recently recorded typed command.
func (f *FakeVoiceService) LastCommand() voiceux.VoiceCommand {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Commands) == 0 {
		return nil
	}
	return f.Commands[len(f.Commands)-1]
}

// LastAction returns the most recently recorded generic action.
func (f *FakeVoiceService) LastAction() (ActionRecord, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Actions) == 0 {
		return ActionRecord{}, false
	}
	return f.Actions[len(f.Actions)-1], true
}
