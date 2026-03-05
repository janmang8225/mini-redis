package persistence

import (
	"log/slog"
	"time"
)

// StoreSnapshot is what the store must implement for the manager to snapshot it.
type StoreSnapshot interface {
	Snapshot() map[string]SnapshotEntry
	LoadSnapshot(entries map[string]SnapshotEntry)
}

// StoreAOF is what the store must implement for AOF replay.
type StoreAOF interface {
	ReplayCommand(args []string) error
}

// Manager coordinates AOF writing and periodic snapshotting.
type Manager struct {
	aof              *AOF
	snapshotPath     string
	snapshotInterval time.Duration
	aofEnabled       bool
	snapshotEnabled  bool
}

// Config holds persistence configuration.
type Config struct {
	AOFEnabled       bool
	AOFPath          string
	SnapshotEnabled  bool
	SnapshotPath     string
	SnapshotInterval time.Duration
}

// NewManager creates a persistence manager. Call Restore() before Start().
func NewManager(cfg Config) (*Manager, error) {
	m := &Manager{
		snapshotPath:     cfg.SnapshotPath,
		snapshotInterval: cfg.SnapshotInterval,
		aofEnabled:       cfg.AOFEnabled,
		snapshotEnabled:  cfg.SnapshotEnabled,
	}

	if cfg.AOFEnabled {
		aof, err := OpenAOF(cfg.AOFPath)
		if err != nil {
			return nil, err
		}
		m.aof = aof
	}

	return m, nil
}

// Restore loads persisted data into the store on startup.
// Order: snapshot first (fast bulk load), then AOF replay (apply recent writes).
func (m *Manager) Restore(store StoreSnapshot, aofStore StoreAOF) error {
	// 1. load snapshot
	if m.snapshotEnabled {
		entries, err := LoadSnapshot(m.snapshotPath)
		if err != nil {
			slog.Warn("snapshot load failed — starting fresh", "err", err)
		} else if len(entries) > 0 {
			store.LoadSnapshot(entries)
			slog.Info("snapshot restored", "keys", len(entries))
		}
	}

	// 2. replay AOF on top of snapshot
	if m.aofEnabled && m.aof != nil {
		n, err := Replay(m.aof.path, aofStore.ReplayCommand)
		if err != nil {
			slog.Warn("aof replay error", "err", err)
		}
		if n > 0 {
			slog.Info("aof replayed", "commands", n)
		}
	}

	return nil
}

// WriteAOF appends a command to the AOF. Called after every successful write.
func (m *Manager) WriteAOF(args []string) {
	if !m.aofEnabled || m.aof == nil {
		return
	}
	if err := m.aof.Write(args); err != nil {
		slog.Error("aof write failed", "err", err)
	}
}

// Start launches the background snapshot ticker.
func (m *Manager) Start(store StoreSnapshot, done <-chan struct{}) {
	if !m.snapshotEnabled || m.snapshotInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(m.snapshotInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				entries := store.Snapshot()
				if err := SaveSnapshot(m.snapshotPath, entries); err != nil {
					slog.Error("snapshot failed", "err", err)
				}
			case <-done:
				// final snapshot on shutdown
				slog.Info("saving final snapshot before exit...")
				entries := store.Snapshot()
				if err := SaveSnapshot(m.snapshotPath, entries); err != nil {
					slog.Error("final snapshot failed", "err", err)
				}
				return
			}
		}
	}()
}

// Close shuts down the manager cleanly.
func (m *Manager) Close() {
	if m.aof != nil {
		m.aof.Close()
	}
}