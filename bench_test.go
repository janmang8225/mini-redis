// Package bench provides benchmarks for the mini-redis store layer.
//
// Run with:
//
//	go test ./bench/... -bench=. -benchmem -benchtime=5s
//
// To compare against real Redis, start Redis on :6380 and run:
//
//	go test ./bench/... -bench=. -benchmem -benchtime=5s -tags redis
package bench

import (
	"fmt"
	"testing"
	"time"

	"github.com/janmang8225/mini-redis/internal/store"
)

// shared store instance — created once, reused across all benchmarks
var benchStore *store.Store

func init() {
	benchStore = store.New()
}

// ── String ops ─────────────────────────────────────────────────────────────

func BenchmarkSet(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.SetString("bench:key", "hello", 0)
	}
}

func BenchmarkGet(b *testing.B) {
	benchStore.SetString("bench:get", "world", 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.GetString("bench:get")
	}
}

func BenchmarkSetWithTTL(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.SetString("bench:ttl", "hello", 60*time.Second)
	}
}

func BenchmarkGetMiss(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.GetString("bench:nonexistent")
	}
}

func BenchmarkIncrBy(b *testing.B) {
	benchStore.SetString("bench:counter", "0", 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.IncrBy("bench:counter", 1)
	}
}

// ── Hash ops ───────────────────────────────────────────────────────────────

func BenchmarkHSet(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.HSet("bench:hash", map[string]string{
			"field1": "value1",
			"field2": "value2",
		})
	}
}

func BenchmarkHGet(b *testing.B) {
	benchStore.HSet("bench:hget", map[string]string{"name": "jan"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.HGet("bench:hget", "name")
	}
}

func BenchmarkHGetAll(b *testing.B) {
	fields := make(map[string]string, 20)
	for i := 0; i < 20; i++ {
		fields[fmt.Sprintf("field%d", i)] = fmt.Sprintf("value%d", i)
	}
	benchStore.HSet("bench:hgetall", fields)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.HGetAll("bench:hgetall")
	}
}

// ── List ops ───────────────────────────────────────────────────────────────

func BenchmarkLPush(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.LPush("bench:list", "item")
	}
}

func BenchmarkRPush(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.RPush("bench:rlist", "item")
	}
}

func BenchmarkLRange100(b *testing.B) {
	for i := 0; i < 100; i++ {
		benchStore.RPush("bench:lrange", fmt.Sprintf("item%d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.LRange("bench:lrange", 0, 99)
	}
}

// ── Set ops ────────────────────────────────────────────────────────────────

func BenchmarkSAdd(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.SAdd("bench:set", fmt.Sprintf("member%d", i))
	}
}

func BenchmarkSIsMember(b *testing.B) {
	benchStore.SAdd("bench:sismember", "go", "redis", "backend")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStore.SIsMember("bench:sismember", "go")
	}
}

// ── Parallel / concurrency ─────────────────────────────────────────────────

func BenchmarkSetParallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchStore.SetString("bench:parallel", "value", 0)
		}
	})
}

func BenchmarkGetParallel(b *testing.B) {
	benchStore.SetString("bench:getparallel", "value", 0)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchStore.GetString("bench:getparallel")
		}
	})
}

func BenchmarkMixedReadWrite(b *testing.B) {
	benchStore.SetString("bench:mixed", "0", 0)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%4 == 0 {
				// 25% writes
				benchStore.SetString("bench:mixed", "value", 0)
			} else {
				// 75% reads
				benchStore.GetString("bench:mixed")
			}
			i++
		}
	})
}