package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics holds all runtime statistics for the server.
// Atomic ops only — safe to read/write from any goroutine with zero contention.
type Metrics struct {
	startTime    time.Time
	cmdTotal     atomic.Int64
	cmdLastSec   atomic.Int64
	cmdThisSec   atomic.Int64
	connTotal    atomic.Int64
	connCurrent  atomic.Int64
}

var Global = &Metrics{
	startTime: time.Now(),
}

func init() {
	// background goroutine — rolls cmdThisSec into cmdLastSec every second
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			current := Global.cmdThisSec.Swap(0)
			Global.cmdLastSec.Store(current)
		}
	}()
}

// RecordCommand increments command counters. Called after every command.
func RecordCommand() {
	Global.cmdTotal.Add(1)
	Global.cmdThisSec.Add(1)
}

// ConnOpened increments connection counters.
func ConnOpened() {
	Global.connTotal.Add(1)
	Global.connCurrent.Add(1)
}

// ConnClosed decrements current connection count.
func ConnClosed() {
	Global.connCurrent.Add(-1)
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		UptimeSeconds:  int64(time.Since(m.startTime).Seconds()),
		CmdTotal:       m.cmdTotal.Load(),
		CmdPerSec:      m.cmdLastSec.Load(),
		ConnTotal:      m.connTotal.Load(),
		ConnCurrent:    m.connCurrent.Load(),
	}
}

// Snapshot is a point-in-time copy of metrics — safe to serialize.
type Snapshot struct {
	UptimeSeconds int64 `json:"uptime_seconds"`
	CmdTotal      int64 `json:"cmd_total"`
	CmdPerSec     int64 `json:"cmd_per_sec"`
	ConnTotal     int64 `json:"conn_total"`
	ConnCurrent   int64 `json:"conn_current"`
}