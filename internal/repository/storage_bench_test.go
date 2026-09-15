package repository

import (
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkFileStorage_Insert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.json")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		store := NewFileStorage()
		if err := store.Init(path); err != nil {
			b.Fatal(err)
		}
		original := "https://example.com/bench/" + strconv.Itoa(i)
		short := "short" + strconv.Itoa(i)
		_ = store.Insert("user-1", original, short)
	}
}

func BenchmarkFileStorage_BatchInsert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.json")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		store := NewFileStorage()
		if err := store.Init(path); err != nil {
			b.Fatal(err)
		}
		records := []URLRecord{
			{OriginalURL: "https://a.com/" + strconv.Itoa(i), ShortURL: "a" + strconv.Itoa(i)},
			{OriginalURL: "https://b.com/" + strconv.Itoa(i), ShortURL: "b" + strconv.Itoa(i)},
			{OriginalURL: "https://c.com/" + strconv.Itoa(i), ShortURL: "c" + strconv.Itoa(i)},
		}
		_, _ = store.BatchInsert("user-1", records)
	}
}

func BenchmarkFileStorage_Select(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.json")

	store := NewFileStorage()
	if err := store.Init(path); err != nil {
		b.Fatal(err)
	}
	_ = store.Insert("user-1", "https://example.com/bench", "short1")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = store.Select("short1")
	}
}
