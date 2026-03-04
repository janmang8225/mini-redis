package commands

import (
	"fmt"
	"strconv"

	"github.com/janmang8225/mini-redis/internal/resp"
	"github.com/janmang8225/mini-redis/internal/store"
)

func handleLists(h *Handler, cmd *resp.Command, w *resp.Writer) bool {
	switch cmd.Name() {
	case "LPUSH":
		lpush(h.store, cmd, w)
	case "RPUSH":
		rpush(h.store, cmd, w)
	case "LPOP":
		lpop(h.store, cmd, w)
	case "RPOP":
		rpop(h.store, cmd, w)
	case "LRANGE":
		lrange(h.store, cmd, w)
	case "LLEN":
		llen(h.store, cmd, w)
	default:
		return false
	}
	return true
}

func lpush(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'LPUSH'")
		return
	}
	n, err := st.LPush(cmd.Args[1], cmd.Args[2:]...)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

func rpush(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'RPUSH'")
		return
	}
	n, err := st.RPush(cmd.Args[1], cmd.Args[2:]...)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

func lpop(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'LPOP'")
		return
	}
	val, ok, err := st.LPop(cmd.Args[1])
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

func rpop(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'RPOP'")
		return
	}
	val, ok, err := st.RPop(cmd.Args[1])
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

func lrange(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 4 {
		_ = w.WriteError("wrong number of arguments for 'LRANGE'")
		return
	}
	start, err1 := strconv.Atoi(cmd.Args[2])
	stop, err2 := strconv.Atoi(cmd.Args[3])
	if err1 != nil || err2 != nil {
		_ = w.WriteError("value is not an integer or out of range")
		return
	}
	items, err := st.LRange(cmd.Args[1], start, stop)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteArrayBulkStrings(items)
}

func llen(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError(fmt.Sprintf("wrong number of arguments for 'LLEN'"))
		return
	}
	n, err := st.LLen(cmd.Args[1])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}