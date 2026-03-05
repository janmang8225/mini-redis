# mini-redis

A Redis-compatible, high-performance in-memory store written in Go.

**Drop-in Redis replacement · Built-in web dashboard · Single binary · Zero dependencies**

```
any Redis client → mini-redis :6379 → dashboard :8080
```

---

## Why mini-redis?

| | Redis | mini-redis |
|---|---|---|
| Protocol | RESP2 | RESP2 ✓ (fully compatible) |
| Web dashboard | ✗ (separate 200MB app) | ✓ built-in at :8080 |
| Config | redis.conf (complex) | single config.yaml |
| Binary size | ~10MB + dependencies | ~10MB, zero dependencies |
| Deployment | install or Docker | `docker run` or single binary |
| Embed in Go app | ✗ | ✓ (coming soon) |

Any Redis client works out of the box — `redis-cli`, `ioredis`, `go-redis`, `jedis`.  
Just point it at port `6379`.

---

## Quickstart

### Binary

```bash
git clone https://github.com/janmang8225/mini-redis
cd mini-redis
go build -o miniredis ./cmd/miniredis
./miniredis
```

### Docker

```bash
docker build -t miniredis .
docker run -p 6379:6379 -p 8080:8080 miniredis
```

### Docker Compose

```bash
docker compose up
```

Then open **http://localhost:8080** for the live dashboard.

---

## Connect

```bash
# redis-cli (interactive mode)
redis-cli
127.0.0.1:6379> ping
PONG
127.0.0.1:6379> set name "jan"
OK
127.0.0.1:6379> get name
"jan"

# any Redis client, zero changes
const client = new Redis({ host: 'localhost', port: 6379 })
await client.set('foo', 'bar')
```

---

## Supported Commands

### Strings
| Command | Description |
|---|---|
| `SET key value [EX s] [PX ms] [NX] [XX]` | Set a string value with optional TTL and conditions |
| `GET key` | Get a string value |
| `DEL key [key ...]` | Delete one or more keys |
| `EXISTS key [key ...]` | Check if keys exist |
| `EXPIRE key seconds` | Set TTL on a key |
| `TTL key` | Get remaining TTL |
| `INCR key` | Increment integer value by 1 |
| `DECR key` | Decrement integer value by 1 |
| `INCRBY key n` | Increment by n |
| `DECRBY key n` | Decrement by n |
| `MSET key val [key val ...]` | Set multiple keys |
| `MGET key [key ...]` | Get multiple keys |

### Lists
| Command | Description |
|---|---|
| `LPUSH key val [val ...]` | Prepend values to list |
| `RPUSH key val [val ...]` | Append values to list |
| `LPOP key` | Remove and return first element |
| `RPOP key` | Remove and return last element |
| `LRANGE key start stop` | Get range of elements |
| `LLEN key` | Get list length |

### Hashes
| Command | Description |
|---|---|
| `HSET key field val [field val ...]` | Set hash fields |
| `HGET key field` | Get a hash field |
| `HDEL key field [field ...]` | Delete hash fields |
| `HGETALL key` | Get all fields and values |
| `HLEN key` | Number of fields |
| `HEXISTS key field` | Check if field exists |

### Sets
| Command | Description |
|---|---|
| `SADD key member [member ...]` | Add members to set |
| `SMEMBERS key` | Get all members |
| `SREM key member [member ...]` | Remove members |
| `SISMEMBER key member` | Check membership |
| `SCARD key` | Get set size |

### Pub/Sub
| Command | Description |
|---|---|
| `SUBSCRIBE channel [channel ...]` | Subscribe to channels |
| `PUBLISH channel message` | Publish a message |

### Server
| Command | Description |
|---|---|
| `PING [message]` | Ping the server |
| `DBSIZE` | Number of keys |
| `FLUSHALL` | Delete all keys |
| `INFO` | Server information |
| `TYPE key` | Get key type |

---

## Configuration

```yaml
port: 6379
log_level: info       # debug | info
max_memory: 256mb

persistence:
  aof: true
  aof_file: miniredis.aof
  snapshot: true
  snapshot_file: miniredis.rdb
  snapshot_interval_seconds: 300

dashboard:
  enabled: true
  port: 8080
```

---

## Persistence

mini-redis uses the same dual-strategy as Redis:

**AOF (Append Only File)** — every write command is fsynced to disk immediately.
On restart, the log is replayed to restore full state. Zero data loss.

**Snapshot** — full binary store dump every N seconds (configurable).
Fast startup. On crash, may lose writes since last snapshot.

Both run together by default. Snapshot for fast startup, AOF for durability.

On graceful shutdown (`Ctrl+C`), a final snapshot is always saved.

---

## Dashboard

Open **http://localhost:8080** after starting mini-redis.

- Live key browser — all keys, types, TTLs
- Key inspector — click any key to see its full value
- Server stats — commands/sec, active clients, uptime, pub/sub subscribers
- Auto-refreshes every 2 seconds

No setup, no separate install. It's baked into the binary.

---

## Benchmarks

Run against the store layer directly (no network overhead):

```bash
go test ./bench/... -bench=. -benchmem -benchtime=5s
```

Typical results on Apple M2:

```
BenchmarkSet-8              ~120 ns/op     48 B/op    1 allocs/op
BenchmarkGet-8               ~55 ns/op      0 B/op    0 allocs/op
BenchmarkSetWithTTL-8       ~135 ns/op     48 B/op    1 allocs/op
BenchmarkGetMiss-8           ~30 ns/op      0 B/op    0 allocs/op
BenchmarkIncrBy-8           ~140 ns/op     24 B/op    2 allocs/op
BenchmarkHSet-8             ~250 ns/op    112 B/op    3 allocs/op
BenchmarkHGet-8              ~65 ns/op      0 B/op    0 allocs/op
BenchmarkLPush-8            ~180 ns/op     64 B/op    2 allocs/op
BenchmarkSAdd-8             ~160 ns/op     48 B/op    1 allocs/op
BenchmarkSetParallel-8       ~45 ns/op     48 B/op    1 allocs/op
BenchmarkGetParallel-8       ~18 ns/op      0 B/op    0 allocs/op
BenchmarkMixedReadWrite-8    ~22 ns/op     12 B/op    0 allocs/op
```

GET at **~18ns** under parallel load. SET at **~45ns** parallel.

---

## Architecture

```
cmd/miniredis/
  main.go               entry point — wires everything

internal/
  server/               TCP listener, goroutine-per-client
  resp/                 RESP2 protocol parser + writer
  store/                thread-safe in-memory store (RWMutex)
  commands/             command handlers (strings, lists, hashes, sets, pubsub)
  persistence/          AOF writer + snapshot encoder/decoder
  pubsub/               goroutine broadcast broker
  metrics/              atomic counters (cmd/sec, connections, uptime)
  dashboard/            embedded HTTP server + REST API

bench/                  Go benchmarks
config/                 YAML config loader
```

**Key design decisions:**

- **Goroutine per connection** — Go's scheduler makes this cheap. 10k concurrent clients = 10k goroutines, no problem.
- **RWMutex on store** — multiple concurrent readers, serialized writers. Read-heavy workloads scale linearly.
- **Atomic metrics** — zero-lock stats collection. `sync/atomic` on the hot path, not `sync.Mutex`.
- **Pub/sub via channels** — no shared memory between publisher and subscribers. Pure message passing.
- **AOF fsync per write** — maximum durability. Configurable to async fsync if throughput matters more.
- **Snapshot via atomic rename** — temp file + rename. Crash during snapshot never corrupts the previous good snapshot.
- **go:embed** — dashboard HTML baked into the binary at compile time. Single file deployment.

---

## License

MIT