package store

import (
	"testing"
)

func BenchmarkCollectionStore_Insert_1000(b *testing.B) {
	ident := func(i int) ItemID { return ItemID(i) }
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s := NewCollectionStore(ident)
		for _, item := range items {
			s.Insert(item)
		}
	}
}

func BenchmarkCollectionStore_Replace_1000(b *testing.B) {
	ident := func(i int) ItemID { return ItemID(i) }
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s := NewCollectionStore(ident)
		for _, item := range items {
			s.Insert(item)
		}
		replacement := make([]int, 1000)
		for j := range replacement {
			replacement[j] = j + 1000
		}
		s.Replace(replacement)
	}
}
