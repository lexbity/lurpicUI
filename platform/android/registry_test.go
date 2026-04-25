package android_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform/android"
	api29 "codeburg.org/lexbit/lurpicui/platform/android/api29"
	api33 "codeburg.org/lexbit/lurpicui/platform/android/api33"
)

func TestRegistrySelectImplementation(t *testing.T) {
	android.ResetRegistryForTest()
	api29.Register()
	api33.Register()

	if got := android.RegisteredAPIs(); len(got) != 2 || got[0] != 29 || got[1] != 33 {
		t.Fatalf("registered APIs = %#v", got)
	}

	impl, ok := android.SelectImplementation(29)
	if !ok || impl == nil || impl.APILevel() != 29 {
		t.Fatalf("target 29 => %#v ok=%v", impl, ok)
	}
	if impl.Storage() == nil || !impl.Storage().UsesScopedStorage() {
		t.Fatal("api29 storage capability missing")
	}
	if impl.BackHandler() == nil || impl.BackHandler().SupportsPredictiveBack() {
		t.Fatal("api29 back handler should not support predictive back")
	}

	impl, ok = android.SelectImplementation(33)
	if !ok || impl == nil || impl.APILevel() != 33 {
		t.Fatalf("target 33 => %#v ok=%v", impl, ok)
	}
	if impl.BackHandler() == nil || !impl.BackHandler().SupportsPredictiveBack() {
		t.Fatal("api33 back handler should support predictive back")
	}

	impl, ok = android.ActiveImplementation()
	if !ok || impl == nil || impl.APILevel() != 33 {
		t.Fatalf("active implementation => %#v ok=%v", impl, ok)
	}
}

func TestRegistryMissingImplementation(t *testing.T) {
	android.ResetRegistryForTest()
	if _, ok := android.SelectImplementation(29); ok {
		t.Fatal("expected no implementation after reset")
	}
}
