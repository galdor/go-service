package shttp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
)

// Reference: https://html.spec.whatwg.org/multipage/server-sent-events.html

type SSEReader struct {
	buf *bufio.Reader
}

type SSEEvent struct {
	// Note that all fields are optional

	Id    string
	Type  string
	Data  string
	Retry int // milliseconds
}

func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		buf: bufio.NewReader(r),
	}
}

func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	// See 9.2.6.

	var event SSEEvent

	for {
		line, err := r.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}

			return nil, err
		}

		if len(line) == 0 {
			// Event end
			return &event, nil
		}

		if line[0] == ':' {
			// Comment
			continue
		}

		fieldData, valueData, _ := bytes.Cut(line, []byte{':'})
		field := string(fieldData)
		value := string(bytes.TrimLeft(valueData, " "))

		switch field {
		case "event":
			event.Type = value

		case "data":
			event.Data = value

		case "id":
			event.Id = value

		case "retry":
			// We are supposed to ignore invalid retry values, but let us be
			// strict and avoid stupid mistakes.
			i64, err := strconv.ParseInt(value, 10, 64)
			if err != nil || i64 < 0 {
				return nil, fmt.Errorf("invalid retry delay %q", value)
			} else if i64 > math.MaxInt {
				return nil, fmt.Errorf("retry delay %d too large (max: %d)",
					i64, math.MaxInt)
			}

		default:
			// Unknown fields are supposed to be ignored, but again let us be
			// careful.
			return nil, fmt.Errorf("unknown field %q", field)
		}
	}

	return nil, nil
}

func (r *SSEReader) readLine() ([]byte, error) {
	// We have to support three possible line ending sequences because SSE is
	// stupid. So it cannot really be made efficient.

	var line []byte
	var i int

	for {
		c, err := r.buf.ReadByte()
		if err != nil {
			return nil, err
		}

		if c == '\n' {
			break
		} else if c == '\r' {
			c2, err := r.buf.Peek(1)
			if err != nil {
				return nil, err
			}

			if c2[0] == '\n' {
				r.buf.Discard(1)
				i++
			}
		}

		line = append(line, c)
		i++
	}

	return line, nil
}
