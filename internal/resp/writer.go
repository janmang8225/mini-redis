package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Writer wraps a bufio.Writer and writes RESP2-encoded responses.
type Writer struct {
	wr *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriter(w)}
}

// WriteSimpleString writes "+OK\r\n" style responses.
func (w *Writer) WriteSimpleString(s string) error {
	_, err := fmt.Fprintf(w.wr, "+%s\r\n", s)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteError writes "-ERR message\r\n" style responses.
func (w *Writer) WriteError(msg string) error {
	_, err := fmt.Fprintf(w.wr, "-ERR %s\r\n", msg)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteWrongType writes the standard Redis wrong type error.
func (w *Writer) WriteWrongType() error {
	_, err := fmt.Fprintf(w.wr, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n")
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteInteger writes ":42\r\n" style responses.
func (w *Writer) WriteInteger(n int64) error {
	_, err := fmt.Fprintf(w.wr, ":%d\r\n", n)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteBulkString writes "$3\r\nfoo\r\n" style responses.
func (w *Writer) WriteBulkString(s string) error {
	_, err := fmt.Fprintf(w.wr, "$%d\r\n%s\r\n", len(s), s)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteNull writes "$-1\r\n" — used when a key doesn't exist.
func (w *Writer) WriteNull() error {
	_, err := fmt.Fprintf(w.wr, "$-1\r\n")
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteNullArray writes "*-1\r\n" — null array.
func (w *Writer) WriteNullArray() error {
	_, err := fmt.Fprintf(w.wr, "*-1\r\n")
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteArray writes the array header "*N\r\n".
// Caller is responsible for writing N elements after this.
func (w *Writer) WriteArrayHeader(count int) error {
	_, err := fmt.Fprintf(w.wr, "*%d\r\n", count)
	return err
}

// WriteArrayBulkStrings writes a complete array of bulk strings in one call.
func (w *Writer) WriteArrayBulkStrings(items []string) error {
	if err := w.WriteArrayHeader(len(items)); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(w.wr, "$%d\r\n%s\r\n", len(item), item); err != nil {
			return err
		}
	}
	return w.wr.Flush()
}

// WriteRaw writes a raw pre-formatted RESP string directly.
// Use sparingly — only when you need precise control.
func (w *Writer) WriteRaw(data string) error {
	_, err := io.WriteString(w.wr, data)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteIntegerArray writes an array of integers.
func (w *Writer) WriteIntegerArray(nums []int64) error {
	if err := w.WriteArrayHeader(len(nums)); err != nil {
		return err
	}
	for _, n := range nums {
		if _, err := fmt.Fprintf(w.wr, ":%s\r\n", strconv.FormatInt(n, 10)); err != nil {
			return err
		}
	}
	return w.wr.Flush()
}