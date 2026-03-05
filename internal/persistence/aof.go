package persistence

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// AOF (Append Only File) writes every mutating command to disk as RESP.
// On startup, we replay the file to restore state.
//
// Format: standard RESP arrays, one command per entry.
// Example entry for SET foo bar:
//   *3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
type AOF struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// OpenAOF opens (or creates) the AOF file for appending.
func OpenAOF(path string) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("aof: failed to open %s: %w", path, err)
	}
	return &AOF{file: f, path: path}, nil
}

// Write appends a command to the AOF file as a RESP array.
// Fsync is called after every write — this is the "always" fsync policy.
// Trade-off: max durability, slight write overhead. Fine for our use case.
func (a *AOF) Write(args []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	encoded := encodeRESP(args)
	if _, err := a.file.WriteString(encoded); err != nil {
		return fmt.Errorf("aof: write failed: %w", err)
	}

	// fsync — guarantee it's on disk, not just in OS buffer
	return a.file.Sync()
}

// Close flushes and closes the AOF file.
func (a *AOF) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}

// Replay reads the AOF file and calls fn for each command.
// fn receives the command args e.g. ["SET", "foo", "bar"].
// Returns the number of commands replayed.
func Replay(path string, fn func(args []string) error) (int, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil // no AOF yet — fresh start
	}
	if err != nil {
		return 0, fmt.Errorf("aof: open for replay failed: %w", err)
	}
	defer f.Close()

	var count int
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer

	for {
		args, err := readRESPCommand(scanner)
		if err != nil {
			break // EOF or parse error — stop replay
		}
		if len(args) == 0 {
			continue
		}
		if err := fn(args); err != nil {
			slog.Warn("aof: replay command failed", "cmd", args[0], "err", err)
			continue // skip bad command, keep going
		}
		count++
	}

	return count, nil
}

// encodeRESP encodes a command as a RESP array string.
func encodeRESP(args []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return sb.String()
}

// readRESPCommand reads one RESP array command from a bufio.Scanner.
// Returns nil args on EOF.
func readRESPCommand(scanner *bufio.Scanner) ([]string, error) {
	// read array header: *N
	if !scanner.Scan() {
		return nil, fmt.Errorf("EOF")
	}
	line := scanner.Text()
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("aof: expected array, got %q", line)
	}

	var count int
	if _, err := fmt.Sscanf(line[1:], "%d", &count); err != nil {
		return nil, fmt.Errorf("aof: bad array count")
	}

	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// read bulk string header: $N
		if !scanner.Scan() {
			return nil, fmt.Errorf("aof: unexpected EOF in bulk header")
		}
		// read bulk string value
		if !scanner.Scan() {
			return nil, fmt.Errorf("aof: unexpected EOF in bulk value")
		}
		args = append(args, scanner.Text())
	}

	return args, nil
}