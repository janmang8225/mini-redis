package store

import (
	"log/slog"
	"time"
)

// StartExpiryWorker launches a background goroutine that actively deletes
// expired keys every `interval`. This is the same dual-strategy Redis uses:
//   - Lazy expiry:  check on read (already in store.get)
//   - Active expiry: this worker sweeps periodically
//
// The done channel stops the worker on graceful shutdown.
func (s *Store) StartExpiryWorker(interval time.Duration, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				n := s.DeleteExpired()
				if n > 0 {
					slog.Debug("active expiry", "deleted", n)
				}
			case <-done:
				slog.Debug("expiry worker stopped")
				return
			}
		}
	}()
}