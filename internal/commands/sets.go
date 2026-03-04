package commands

import (
	"github.com/janmang8225/mini-redis/internal/resp"
	"github.com/janmang8225/mini-redis/internal/store"
)

func handleSets(h *Handler, cmd *resp.Command, w *resp.Writer) bool {
	switch cmd.Name() {
	case "SADD":
		sadd(h.store, cmd, w)
	case "SMEMBERS":
		smembers(h.store, cmd, w)
	case "SREM":
		srem(h.store, cmd, w)
	case "SISMEMBER":
		sismember(h.store, cmd, w)
	case "SCARD":
		scard(h.store, cmd, w)
	default:
		return false
	}
	return true
}

func sadd(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'SADD'")
		return
	}
	n, err := st.SAdd(cmd.Args[1], cmd.Args[2:]...)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

func smembers(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'SMEMBERS'")
		return
	}
	members, err := st.SMembers(cmd.Args[1])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteArrayBulkStrings(members)
}

func srem(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 3 {
		_ = w.WriteError("wrong number of arguments for 'SREM'")
		return
	}
	n, err := st.SRem(cmd.Args[1], cmd.Args[2:]...)
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}

func sismember(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError("wrong number of arguments for 'SISMEMBER'")
		return
	}
	ok, err := st.SIsMember(cmd.Args[1], cmd.Args[2])
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

func scard(st *store.Store, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 2 {
		_ = w.WriteError("wrong number of arguments for 'SCARD'")
		return
	}
	n, err := st.SCard(cmd.Args[1])
	if err != nil {
		_ = w.WriteError(err.Error())
		return
	}
	_ = w.WriteInteger(n)
}