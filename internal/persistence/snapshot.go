package persistence

import (
	"encoding/gob"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// SnapshotEntry is the serializable form of a store entry.
// We use gob encoding — binary, fast, Go-native.
type SnapshotEntry struct {
	Type      uint8  // 0=string, 1=list, 2=hash, 3=set
	StrVal    string
	ListVal   []string
	HashVal   map[string]string
	SetVal    []string // stored as slice, rebuilt as map on load
	ExpiresAt time.Time
}

// SnapshotData is the full snapshot written to disk.
type SnapshotData struct {
	CreatedAt time.Time
	Entries   map[string]SnapshotEntry
}

// SaveSnapshot writes the full store snapshot to a temp file then atomically
// renames it to the target path. This prevents corrupt snapshots on crash.
func SaveSnapshot(path string, entries map[string]SnapshotEntry) error {
	// write to temp file first
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("snapshot: create tmp failed: %w", err)
	}

	data := SnapshotData{
		CreatedAt: time.Now(),
		Entries:   entries,
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("snapshot: encode failed: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("snapshot: sync failed: %w", err)
	}
	f.Close()

	// atomic rename — either old or new, never corrupt
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("snapshot: rename failed: %w", err)
	}

	slog.Info("snapshot saved", "path", path, "keys", len(entries))
	return nil
}

// LoadSnapshot reads and decodes a snapshot file.
// Returns empty map if file doesn't exist (fresh start).
func LoadSnapshot(path string) (map[string]SnapshotEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]SnapshotEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot: open failed: %w", err)
	}
	defer f.Close()

	var data SnapshotData
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("snapshot: decode failed: %w", err)
	}

	slog.Info("snapshot loaded", "path", path, "keys", len(data.Entries), "created", data.CreatedAt)
	return data.Entries, nil
}