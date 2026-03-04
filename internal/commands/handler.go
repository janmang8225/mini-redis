package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janmang8225/mini-redis/internal/resp"
	"github.com/janmang8225/mini-redis/internal/store"
)

// Handler dispatches incoming commands to their implementations.
type Handler struct {
	store *store.Store
}

func NewHandler(st *store.Store) *Handler {
	return &Handler{store: st}
}

// Handle routes a parsed command to the correct function.
func (h *Handler) Handle(cmd *resp.Command, w *resp.Writer) {
	switch cmd.Name() {

	// --- connection ---
	case "PING":
		h.ping(cmd, w)
	case "QUIT":
		_ = w.WriteSimpleString("OK")

	// --- server ---
	case "FLUSHALL":
		h.flushAll(cmd, w)
	case "DBSIZE":
		h.dbSize(cmd, w)
	case "INFO":
		h.info(cmd, w)

	// --- strings ---
	case "SET":
		h.set(cmd, w)
	case "GET":
		h.get(cmd, w)
	case "DEL":
		h.del(cmd, w)
	case "EXISTS":
		h.exists(cmd, w)
	case "EXPIRE":
		h.expire(cmd, w)
	case "TTL":
		h.ttl(cmd, w)
	case "INCR":
		h.incrBy(cmd, w, 1)
	case "DECR":
		h.incrBy(cmd, w, -1)
	case "INCRBY":
		h.incrByN(cmd, w, false)
	case "DECRBY":
		h.incrByN(cmd, w, true)
	case "MSET":
		h.mset(cmd, w)
	case "MGET":
		h.mget(cmd, w)
	case "TYPE":
		h.typeCmd(cmd, w)

	default:
		_ = w.WriteError(fmt.Sprintf("unknown command '%s'", cmd.Name()))
	}
}

// ─── Connection ────────────────────────────────────────────────────────────────

func (h *Handler) ping(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) == 2 {
		// PING <message> → returns the message as bulk string
		_ = w.WriteBulkString(cmd.Args[1])
		return
	}
	_ = w.WriteSimpleString("PONG")
}

// ─── Server ────────────────────────────────────────────────────────────────────

func (h *Handler) flushAll(_ *resp.Command, w *resp.Writer) {
	h.store.FlushAll()
	_ = w.WriteSimpleString("OK")
}

func (h *Handler) dbSize(_ *resp.Command, w *resp.Writer) {
	_ = w.WriteInteger(int64(h.store.DBSize()))
}

func (h *Handler) info(_ *resp.Command, w *resp.Writer) {
	info := fmt.Sprintf(
		"# Server\r\nredis_version:7.0.0-miniredis\r\nos:Go\r\n\r\n# Keyspace\r\ndb0:keys=%d\r\n",
		h.store.DBSize(),
	)
	_ = w.WriteBulkString(info)
}

// ─── Strings ───────────────────────────────────────────────────────────────────

// SET key value [EX seconds] [PX milliseconds] [NX] [XX]
func (h *Handler) set(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'SET'")
		return
	}

	key := cmd.Args[1]
	val := cmd.Args[2]
	var ttl time.Duration
	nx, xx := false, false

	// parse options
	for i := 3; i < len(cmd.Args); i++ {
		opt := strings.ToUpper(cmd.Args[i])
		switch opt {
		case "EX":
			if i+1 >= len(cmd.Args) {
				_ = w.WriteError("syntax error")
				return
			}
			secs, err := strconv.ParseInt(cmd.Args[i+1], 10, 64)
			if err != nil || secs <= 0 {
				_ = w.WriteError("invalid expire time in 'SET'")
				return
			}
			ttl = time.Duration(secs) * time.Second
			i++
		case "PX":
			if i+1 >= len(cmd.Args) {
				_ = w.WriteError("syntax error")
				return
			}
			ms, err := strconv.ParseInt(cmd.Args[i+1], 10, 64)
			if err != nil || ms <= 0 {
				_ = w.WriteError("invalid expire time in 'SET'")
				return
			}
			ttl = time.Duration(ms) * time.Millisecond
			i++
		case "NX":
			nx = true
		case "XX":
			xx = true
		}
	}

	// NX — only set if NOT exists
	if nx {
		if _, exists := h.store.GetString(key); exists {
			_ = w.WriteNull()
			return
		}
	}

	// XX — only set if EXISTS
	if xx {
		if _, exists := h.store.GetString(key); !exists {
			_ = w.WriteNull()
			return
		}
	}

	h.store.SetString(key, val, ttl)
	_ = w.WriteSimpleString("OK")
}

func (h *Handler) get(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'GET'")
		return
	}

	val, ok := h.store.GetString(cmd.Args[1])
	if !ok {
		_ = w.WriteNull()
		return
	}
	_ = w.WriteBulkString(val)
}

func (h *Handler) del(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 2 {
		_ = w.WriteError("wrong number of arguments for 'DEL'")
		return
	}
	_ = w.WriteInteger(h.store.Delete(cmd.Args[1:]...))
}

func (h *Handler) exists(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 2 {
		_ = w.WriteError("wrong number of arguments for 'EXISTS'")
		return
	}
	_ = w.WriteInteger(h.store.Exists(cmd.Args[1:]...))
}

func (h *Handler) expire(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError("wrong number of arguments for 'EXPIRE'")
		return
	}
	secs, err := strconv.ParseInt(cmd.Args[2], 10, 64)
	if err != nil || secs <= 0 {
		_ = w.WriteError("invalid expire time")
		return
	}
	ok := h.store.Expire(cmd.Args[1], time.Duration(secs)*time.Second)
	if ok {
		_ = w.WriteInteger(1)
	} else {
		_ = w.WriteInteger(0)
	}
}

func (h *Handler) ttl(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'TTL'")
		return
	}
	d := h.store.TTL(cmd.Args[1])
	switch d {
	case -2:
		_ = w.WriteInteger(-2) // key doesn't exist
	case -1:
		_ = w.WriteInteger(-1) // no expiry
	default:
		_ = w.WriteInteger(int64(d.Seconds()))
	}
}

// INCR / DECR — shared logic
func (h *Handler) incrBy(cmd *resp.Command, w *resp.Writer, delta int64) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError(fmt.Sprintf("wrong number of arguments for '%s'", cmd.Name()))
		return
	}
	key := cmd.Args[1]
	newVal, err := h.store.IncrBy(key, delta)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(newVal)
}

// INCRBY / DECRBY
func (h *Handler) incrByN(cmd *resp.Command, w *resp.Writer, negate bool) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError(fmt.Sprintf("wrong number of arguments for '%s'", cmd.Name()))
		return
	}
	n, err := strconv.ParseInt(cmd.Args[2], 10, 64)
	if err != nil {
		_ = w.WriteError("value is not an integer or out of range")
		return
	}
	if negate {
		n = -n
	}
	newVal, err := h.store.IncrBy(cmd.Args[1], n)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(newVal)
}

// MSET key1 val1 key2 val2 ...
func (h *Handler) mset(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 || len(cmd.Args)%2 == 0 {
		_ = w.WriteError("wrong number of arguments for 'MSET'")
		return
	}
	for i := 1; i < len(cmd.Args); i += 2 {
		h.store.SetString(cmd.Args[i], cmd.Args[i+1], 0)
	}
	_ = w.WriteSimpleString("OK")
}

// MGET key1 key2 ...
func (h *Handler) mget(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 2 {
		_ = w.WriteError("wrong number of arguments for 'MGET'")
		return
	}
	results := make([]string, len(cmd.Args)-1)
	for i, key := range cmd.Args[1:] {
		val, ok := h.store.GetString(key)
		if ok {
			results[i] = val
		}
	}
	// write array manually to handle nulls
	_ = w.WriteArrayHeader(len(results))
	for _, key := range cmd.Args[1:] {
		val, ok := h.store.GetString(key)
		if !ok {
			_ = w.WriteNull()
		} else {
			_ = w.WriteBulkString(val)
		}
	}
}

func (h *Handler) typeCmd(cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'TYPE'")
		return
	}
	_ = w.WriteSimpleString(h.store.GetType(cmd.Args[1]))
}