package repository

import (
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkFileStorage_Insert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.json")

	// Подготовка — не измеряется
	store := NewFileStorage()
	if err := store.Init(path); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	i := 0
	for b.Loop() {
		original := "https://example.com/bench/" + strconv.Itoa(i)
		short := "short" + strconv.Itoa(i)
		_ = store.Insert("user-1", original, short)
		i++
	}
}

func BenchmarkFileStorage_BatchInsert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.json")

	// Подготовка — не измеряется
	store := NewFileStorage()
	if err := store.Init(path); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	i := 0
	for b.Loop() {
		records := []URLRecord{
			{OriginalURL: "https://a.com/" + strconv.Itoa(i), ShortURL: "a" + strconv.Itoa(i)},
			{OriginalURL: "https://b.com/" + strconv.Itoa(i), ShortURL: "b" + strconv.Itoa(i)},
			{OriginalURL: "https://c.com/" + strconv.Itoa(i), ShortURL: "c" + strconv.Itoa(i)},
		}
		_, _ = store.BatchInsert("user-1", records)
		i++
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

	for b.Loop() {
		_, _ = store.Select("short1")
	}
}
