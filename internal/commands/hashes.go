package commands

import (
	"github.com/janmang8225/mini-redis/internal/resp"
	"github.com/janmang8225/mini-redis/internal/store"
)

func handleHashes(h *Handler, cmd *resp.Command, w *resp.Writer) bool {
	switch cmd.Name() {
	case "HSET", "HMSET":
		hset(h.store, cmd, w)
	case "HGET":
		hget(h.store, cmd, w)
	case "HDEL":
		hdel(h.store, cmd, w)
	case "HGETALL":
		hgetall(h.store, cmd, w)
	case "HLEN":
		hlen(h.store, cmd, w)
	case "HEXISTS":
		hexists(h.store, cmd, w)
	default:
		return false
	}
	return true
}

// HSET key field1 val1 [field2 val2 ...]
func hset(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	// minimum: HSET key field value
	if len(cmd.Args) < 4 || len(cmd.Args)%2 != 0 {
		_ = w.WriteError("wrong number of arguments for 'HSET'")
		return
	}
	pairs := make(map[string]string)
	for i := 2; i < len(cmd.Args); i += 2 {
		pairs[cmd.Args[i]] = cmd.Args[i+1]
	}
	n, err := st.HSet(cmd.Args[1], pairs)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	// HMSET always returns OK, HSET returns count of new fields
	if cmd.Name() == "HMSET" {
		_ = w.WriteSimpleString("OK")
	} else {
		_ = w.WriteInteger(n)
	}
}

func hget(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError("wrong number of arguments for 'HGET'")
		return
	}
	val, ok, err := st.HGet(cmd.Args[1], cmd.Args[2])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	if !ok {
		_ = w.WriteNull()
		return
	}
	_ = w.WriteBulkString(val)
}

func hdel(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'HDEL'")
		return
	}
	n, err := st.HDel(cmd.Args[1], cmd.Args[2:]...)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

// HGETALL returns flat array: [field1, val1, field2, val2, ...]
func hgetall(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'HGETALL'")
		return
	}
	hash, err := st.HGetAll(cmd.Args[1])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	// flatten to alternating field/value array
	flat := make([]string, 0, len(hash)*2)
	for f, v := range hash {
		flat = append(flat, f, v)
	}
	_ = w.WriteArrayBulkStrings(flat)
}

func hlen(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'HLEN'")
		return
	}
	n, err := st.HLen(cmd.Args[1])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

func hexists(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError("wrong number of arguments for 'HEXISTS'")
		return
	}
	_, ok, err := st.HGet(cmd.Args[1], cmd.Args[2])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	if ok {
		_ = w.WriteInteger(1)
	} else {
		_ = w.WriteInteger(0)
	}
}