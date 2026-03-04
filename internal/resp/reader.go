package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// ErrInvalidProtocol is returned when the client sends malformed RESP data.
var ErrInvalidProtocol = errors.New("invalid RESP protocol")

// Command represents a parsed Redis command.
// e.g. ["SET", "foo", "bar"]
type Command struct {
	Args []string
}

// Name returns the uppercase command name e.g. "SET"
func (c *Command) Name() string {
	if len(c.Args) == 0 {
		return ""
	}
	name := c.Args[0]
	// uppercase manually — avoids importing strings just for this
	b := []byte(name)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 32
		}
	}
	return string(b)
}

// Reader wraps a bufio.Reader and parses RESP2 commands.
type Reader struct {
	rd *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(r)}
}

// ReadCommand blocks until a full RESP command is received or an error occurs.
// Handles both:
//   - Inline commands: "PING\r\n" (sent by telnet / simple clients)
//   - Array commands:  "*3\r\n$3\r\nSET\r\n..." (sent by all real Redis clients)
func (r *Reader) ReadCommand() (*Command, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, ErrInvalidProtocol
	}

	switch line[0] {
	case '*':
		return r.readArray(line)
	default:
		// inline command — split by spaces
		return parseInline(line), nil
	}
}

// readArray parses "*N\r\n" followed by N bulk strings.
func (r *Reader) readArray(line string) (*Command, error) {
	count, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, fmt.Errorf("%w: bad array count %q", ErrInvalidProtocol, line)
	}

	if count <= 0 {
		return &Command{Args: []string{}}, nil
	}

	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		arg, err := r.readBulkString()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	return &Command{Args: args}, nil
}

// readBulkString parses "$N\r\n<data>\r\n"
func (r *Reader) readBulkString() (string, error) {
	line, err := r.readLine()
	if err != nil {
		return "", err
	}

	if len(line) == 0 || line[0] != '$' {
		return "", fmt.Errorf("%w: expected bulk string, got %q", ErrInvalidProtocol, line)
	}

	length, err := strconv.Atoi(line[1:])
	if err != nil {
		return "", fmt.Errorf("%w: bad bulk string length %q", ErrInvalidProtocol, line)
	}

	if length == -1 {
		return "", nil // null bulk string
	}

	// read exactly `length` bytes + "\r\n"
	buf := make([]byte, length+2)
	if _, err := io.ReadFull(r.rd, buf); err != nil {
		return "", err
	}

	return string(buf[:length]), nil
}

// readLine reads until \r\n and returns the line without the trailing \r\n.
func (r *Reader) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}

	// strip \r\n
	if len(line) >= 2 && line[len(line)-2] == '\r' {
		return line[:len(line)-2], nil
	}

	return line[:len(line)-1], nil
}

// parseInline splits a raw inline command like "PING" or "SET foo bar" by spaces.
func parseInline(line string) *Command {
	args := []string{}
	current := []byte{}

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == ' ' {
			if len(current) > 0 {
				args = append(args, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, ch)
		}
	}

	if len(current) > 0 {
		args = append(args, string(current))
	}

	return &Command{Args: args}
}