//go:build lurpic_debug && !android

package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
)

func TestStore_mutations_require_runtime_thread(t *testing.T) {
	vs := NewValueStore(0)
	ms := NewMapStore[string, int]()
	cs := NewCollectionStore(func(v int) ItemID { return ItemID(v) })

	cases := []struct {
		name   string
		mutate func()
	}{
		{"ValueStore.Set", func() { vs.Set(1) }},
		{"ValueStore.SetTx", func() { vs.SetTx(1, &Transaction{}) }},
		{"MapStore.Set", func() { ms.Set("k", 1) }},
		{"MapStore.Delete", func() { ms.Delete("k") }},
		{"MapStore.Clear", func() { ms.Clear() }},
		{"MapStore.SetTx", func() { ms.SetTx("k", 1, &Transaction{}) }},
		{"MapStore.DeleteTx", func() { ms.DeleteTx("k", &Transaction{}) }},
		{"MapStore.ClearTx", func() { ms.ClearTx(&Transaction{}) }},
		{"CollectionStore.Insert", func() { cs.Insert(1) }},
		{"CollectionStore.Remove", func() { cs.Remove(ItemID(1)) }},
		{"CollectionStore.Update", func() { cs.Update(1) }},
		{"CollectionStore.Replace", func() { cs.Replace([]int{1, 2, 3}) }},
		{"CollectionStore.InsertTx", func() { cs.InsertTx(1, &Transaction{}) }},
		{"CollectionStore.RemoveTx", func() { cs.RemoveTx(ItemID(1), &Transaction{}) }},
		{"CollectionStore.UpdateTx", func() { cs.UpdateTx(1, &Transaction{}) }},
		{"CollectionStore.ReplaceTx", func() { cs.ReplaceTx([]int{1, 2, 3}, &Transaction{}) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/off_thread_panics", func(t *testing.T) {
			syncutil.ResetRuntimeThreadForTest()
			t.Cleanup(syncutil.ResetRuntimeThreadForTest)
			syncutil.RegisterRuntimeThread()
			if r := recoverOffRuntimeThread(t, tc.mutate); r == nil {
				t.Fatalf("expected AssertRuntimeThread panic off the runtime thread")
			}
		})
		t.Run(tc.name+"/on_thread_ok", func(t *testing.T) {
			withRuntimeThread(t, func() { tc.mutate() })
		})
	}
}
