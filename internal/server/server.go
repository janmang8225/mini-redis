package server

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/janmang8225/mini-redis/internal/commands"
	"github.com/janmang8225/mini-redis/internal/persistence"
	"github.com/janmang8225/mini-redis/internal/resp"
	"github.com/janmang8225/mini-redis/internal/store"
)

// Server is the TCP server. One goroutine per connected client.
type Server struct {
	addr     string
	listener net.Listener
	store    *store.Store
	handler  *commands.Handler

	// graceful shutdown
	quit     chan struct{}
	wg       sync.WaitGroup
	once     sync.Once

	// live stats — atomic so no lock needed
	connCount atomic.Int64
}

func New(addr string, st *store.Store, pm *persistence.Manager) *Server {
	return &Server{
		addr:    addr,
		store:   st,
		handler: commands.NewHandler(st, pm),
		quit:    make(chan struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.listener = ln

	slog.Info("miniredis listening", "addr", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			// check if we shut down intentionally
			select {
			case <-s.quit:
				return nil
			default:
				slog.Error("accept error", "err", err)
				continue
			}
		}

		s.connCount.Add(1)
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

// Stop initiates a graceful shutdown — stops accepting, waits for all
// in-flight connections to finish.
func (s *Server) Stop() {
	s.once.Do(func() {
		close(s.quit)
		s.listener.Close()
		s.wg.Wait()
		slog.Info("server stopped cleanly")
	})
}

// ConnCount returns the number of currently connected clients.
func (s *Server) ConnCount() int64 {
	return s.connCount.Load()
}

// handleConn runs in its own goroutine — one per client.
func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.connCount.Add(-1)
		s.wg.Done()
	}()

	clientAddr := conn.RemoteAddr().String()
	slog.Debug("client connected", "addr", clientAddr)

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	for {
		// check if server is shutting down before blocking on read
		select {
		case <-s.quit:
			return
		default:
		}

		cmd, err := reader.ReadCommand()
		if err != nil {
			if errors.Is(err, io.EOF) || isConnectionClosed(err) {
				slog.Debug("client disconnected", "addr", clientAddr)
				return
			}
			// malformed input — tell the client and keep the connection alive
			slog.Warn("protocol error", "addr", clientAddr, "err", err)
			_ = writer.WriteError("Protocol error: " + err.Error())
			continue
		}

		if len(cmd.Args) == 0 {
			continue
		}

		slog.Debug("command received", "addr", clientAddr, "cmd", cmd.Name(), "args", cmd.Args[1:])

		// dispatch to command handler
		s.handler.Handle(cmd, writer)
	}
}

// isConnectionClosed checks for the "use of closed network connection" error
// that Go returns when a connection is closed from another goroutine.
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	return false
}