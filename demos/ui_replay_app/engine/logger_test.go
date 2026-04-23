package engine

import (
	"testing"
	"time"
)

func TestSemanticLogger_Log(t *testing.T) {
	logger := NewSemanticLogger()

	t.Run("logs entry with correct fields", func(t *testing.T) {
		logger.SetStep(5)
		logger.Log(LevelInfo, "test", "test message", map[string]interface{}{"key": "value"})

		entries := logger.Entries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}

		if entries[0].Level != LevelInfo {
			t.Errorf("Level = %v, want %v", entries[0].Level, LevelInfo)
		}
		if entries[0].Category != "test" {
			t.Errorf("Category = %v, want %v", entries[0].Category, "test")
		}
		if entries[0].Message != "test message" {
			t.Errorf("Message = %v, want %v", entries[0].Message, "test message")
		}
		if entries[0].Step != 5 {
			t.Errorf("Step = %d, want 5", entries[0].Step)
		}
		if entries[0].Data["key"] != "value" {
			t.Error("Data missing expected key")
		}
	})

	t.Run("convenience methods work", func(t *testing.T) {
		logger.Clear()

		logger.Debug("cat", "debug msg")
		logger.Info("cat", "info msg")
		logger.Warning("cat", "warning msg")
		logger.Error("cat", "error msg")
		logger.Fatal("cat", "fatal msg")

		entries := logger.Entries()
		if len(entries) != 5 {
			t.Fatalf("Expected 5 entries, got %d", len(entries))
		}

		levels := []LogLevel{LevelDebug, LevelInfo, LevelWarning, LevelError, LevelFatal}
		for i, expected := range levels {
			if entries[i].Level != expected {
				t.Errorf("Entry %d Level = %v, want %v", i, entries[i].Level, expected)
			}
		}
	})
}

func TestSemanticLogger_ActionLogging(t *testing.T) {
	logger := NewSemanticLogger()

	t.Run("action started", func(t *testing.T) {
		logger.ActionStarted("click", 3)

		entries := logger.Entries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}

		if entries[0].Category != "action" {
			t.Errorf("Category = %v, want action", entries[0].Category)
		}
		if entries[0].Data["action_type"] != "click" {
			t.Error("Data missing action_type")
		}
		if entries[0].Data["step"] != 3 {
			t.Error("Data missing step")
		}
	})

	t.Run("action completed", func(t *testing.T) {
		logger.Clear()
		logger.ActionCompleted("click", 3, 100*time.Millisecond)

		entries := logger.Entries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}

		if entries[0].Data["duration_ms"] != int64(100) {
			t.Errorf("duration_ms = %v, want 100", entries[0].Data["duration_ms"])
		}
	})

	t.Run("action failed", func(t *testing.T) {
		logger.Clear()
		err := &testError{msg: "something went wrong"}
		logger.ActionFailed("click", 3, err)

		entries := logger.Entries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}

		if entries[0].Level != LevelError {
			t.Errorf("Level = %v, want error", entries[0].Level)
		}
		if entries[0].Data["error"] != "something went wrong" {
			t.Errorf("error = %v", entries[0].Data["error"])
		}
	})
}

func TestSemanticLogger_AssertionLogging(t *testing.T) {
	logger := NewSemanticLogger()

	t.Run("assertion passed", func(t *testing.T) {
		logger.AssertionChecked("scene_id", 5, true)

		entries := logger.Entries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(entries))
		}

		if entries[0].Level != LevelInfo {
			t.Errorf("Level = %v, want info", entries[0].Level)
		}
		if entries[0].Category != "assertion" {
			t.Error("Expected assertion category")
		}
		if entries[0].Data["passed"] != true {
			t.Error("Expected passed = true")
		}
	})

	t.Run("assertion failed", func(t *testing.T) {
		logger.Clear()
		logger.AssertionChecked("scene_id", 5, false)

		entries := logger.Entries()
		if entries[0].Level != LevelError {
			t.Errorf("Level = %v, want error", entries[0].Level)
		}
		if entries[0].Data["passed"] != false {
			t.Error("Expected passed = false")
		}
	})
}

func TestSemanticLogger_StateLogging(t *testing.T) {
	logger := NewSemanticLogger()

	logger.StateChanged(StateIdle, StateRunning)

	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Category != "state" {
		t.Error("Expected state category")
	}
	if entries[0].Data["from_state"] != StateIdle {
		t.Error("Expected from_state")
	}
	if entries[0].Data["to_state"] != StateRunning {
		t.Error("Expected to_state")
	}
}

func TestSemanticLogger_EventLogging(t *testing.T) {
	logger := NewSemanticLogger()

	logger.EventCaptured("click", "button.ok", map[string]interface{}{"x": 100, "y": 200})

	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Category != "event" {
		t.Error("Expected event category")
	}
	if entries[0].Level != LevelDebug {
		t.Errorf("Level = %v, want debug", entries[0].Level)
	}
	if entries[0].Data["event_type"] != "click" {
		t.Error("Expected event_type")
	}
	if entries[0].Data["x"] != 100 {
		t.Error("Expected x coordinate")
	}
}

func TestSemanticLogger_SummaryLogging(t *testing.T) {
	logger := NewSemanticLogger()

	logger.SummaryCaptured("store", map[string]interface{}{"count": 5, "name": "registry"})

	entries := logger.Entries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Category != "summary" {
		t.Error("Expected summary category")
	}
	summary := entries[0].Data["summary"].(map[string]interface{})
	if summary["count"] != 5 {
		t.Error("Expected count in summary")
	}
}

func TestSemanticLogger_Filtering(t *testing.T) {
	logger := NewSemanticLogger()

	logger.Info("cat1", "msg1")
	logger.Error("cat2", "msg2")
	logger.Info("cat1", "msg3")
	logger.Debug("cat3", "msg4")

	t.Run("filter by level", func(t *testing.T) {
		errors := logger.EntriesByLevel(LevelError)
		if len(errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(errors))
		}
		if errors[0].Message != "msg2" {
			t.Error("Wrong error entry")
		}
	})

	t.Run("filter by category", func(t *testing.T) {
		cat1 := logger.EntriesByCategory("cat1")
		if len(cat1) != 2 {
			t.Errorf("Expected 2 cat1 entries, got %d", len(cat1))
		}
	})

	t.Run("filter by step", func(t *testing.T) {
		logger.SetStep(10)
		logger.Info("step", "at step 10")
		logger.SetStep(11)
		logger.Info("step", "at step 11")

		step10 := logger.EntriesByStep(10)
		if len(step10) != 1 {
			t.Errorf("Expected 1 step 10 entry, got %d", len(step10))
		}
	})
}

func TestSemanticLogger_Clear(t *testing.T) {
	logger := NewSemanticLogger()

	logger.Info("test", "message")
	if logger.Count() != 1 {
		t.Error("Expected 1 entry before clear")
	}

	logger.Clear()
	if logger.Count() != 0 {
		t.Error("Expected 0 entries after clear")
	}
}

func TestSemanticLogger_Format(t *testing.T) {
	logger := NewSemanticLogger()
	logger.SetStep(5)
	logger.Info("action", "test action")

	formatted := logger.Format()
	if formatted == "" {
		t.Error("Format() should not return empty")
	}
	if formatted[:1] != "[" {
		t.Error("Format() should start with timestamp bracket")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
