package writer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ErrDuplicateKey is returned when registering duplicate key.
var ErrDuplicateKey = errors.New("duplicate key")

const (
	streamPrefix     = `\🎏`
	streamJSONPrefix = `"\\🎏`
)

type streamState int

const (
	stateUndetermined streamState = iota
	stateValue
	stateNotValue
)

// ValueFunc is a callback function, in which you can write large JSON value to w.
type ValueFunc func(w io.Writer) error

// ElementWriter encodes and writes array elements.
type ElementWriter interface {
	// WriteElement encodes and writes an array element.
	WriteElement(e interface{}) error
}

// ArrayValueFunc is a callback function, in which you can write each elements of an array to w.
type ArrayValueFunc func(w ElementWriter) error

// Writer writes JSON encoded by json.Encoder.
type Writer struct {
	w io.Writer
	m map[string]*Value
	sync.Mutex

	// states
	onString    bool
	escaping    bool
	streamState streamState
	stringBuf   bytes.Buffer
}

// Value describes future JSON value which is loaded with streaming later.
type Value struct {
	key string
	f   interface{} // ValueFunc or ArrayValueFunc
}

// New creates new Writer which can be passed to json.NewEncoder.
func New(w io.Writer) *Writer {
	return &Writer{
		w: w,
		m: map[string]*Value{},
	}
}

// NewValue creates a Value.
// key can be any string even empty, but must be unique.
// error is returned only when duplicate key indicated.
func (w *Writer) NewValue(key string, f ValueFunc) (*Value, error) {
	return w.newValue(key, f)
}

// MustNewValue creates a Value.
// key can be any string even empty, but must be unique.
// It panics when duplicate key indicated.
func (w *Writer) MustNewValue(key string, f ValueFunc) *Value {
	return w.mustNewValue(key, f)
}

// NewArrayValue creates a Value which describes JSON array.
// key can be any string even empty, but must be unique.
// error is returned only when duplicate key indicated.
func (w *Writer) NewArrayValue(key string, f ArrayValueFunc) (*Value, error) {
	return w.newValue(key, f)
}

// MustNewArrayValue creates a Value which describes JSON array.
// key can be any string even empty, but must be unique.
// It panics when duplicate key indicated.
func (w *Writer) MustNewArrayValue(key string, f ArrayValueFunc) *Value {
	return w.mustNewValue(key, f)
}

func (w *Writer) newValue(key string, f interface{}) (*Value, error) {
	w.Lock()
	defer w.Unlock()

	if _, ok := w.m[key]; ok {
		return nil, ErrDuplicateKey
	}

	v := &Value{
		key: key,
		f:   f,
	}

	w.m[key] = v

	return v, nil
}

func (w *Writer) mustNewValue(key string, f interface{}) *Value {
	v, err := w.newValue(key, f)
	if err != nil {
		panic(err)
	}
	return v
}

func (w *Writer) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if w.onString {
			if w.escaping {
				w.escaping = false
			} else if b == '\\' {
				w.escaping = true
			} else if b == '"' {
				w.onString = false
			}

			if w.streamState == stateNotValue {
				_, err := w.w.Write([]byte{b})
				if err != nil {
					return n, err
				}
			} else {
				_ = w.stringBuf.WriteByte(b)

				if w.streamState == stateUndetermined {
					if w.stringBuf.Len() >= len(streamJSONPrefix) {
						if strings.HasPrefix(w.stringBuf.String(), streamJSONPrefix) {
							w.streamState = stateValue
						} else {
							w.streamState = stateNotValue

							// flush the buffer
							nn, err := w.w.Write(w.stringBuf.Bytes())
							n += nn
							if err != nil {
								return n, err
							}
						}
					}
				}
			}

			if !w.onString {
				// finish string
				if w.streamState == stateUndetermined {
					// flush the buffer
					nn, err := w.w.Write(w.stringBuf.Bytes())
					n += nn
					if err != nil {
						return n, err
					}
				} else if w.streamState == stateValue {
					// process streaming!!
					var s string
					if err := json.Unmarshal(w.stringBuf.Bytes(), &s); err != nil {
						return n, err
					}
					key := s[len(streamPrefix):]

					if err := w.streamValue(key); err != nil {
						return n, err
					}
				}
			}

			continue
		}

		// TODO: process only JSON value strings (now process key strings unnecesarily)
		if b == '"' {
			// start string
			w.onString = true
			w.escaping = false
			w.streamState = stateUndetermined
			w.stringBuf.Reset()
			_ = w.stringBuf.WriteByte('"')
			continue
		}

		_, err := w.w.Write([]byte{b})
		if err != nil {
			return n, err
		}
		n++
	}

	return n, nil
}

func (w *Writer) streamValue(key string) error {

	v, ok := w.m[key]
	if !ok {
		return fmt.Errorf("unexpected key: %s", key)
	}

	switch f := v.f.(type) {
	case ValueFunc:
		if err := f(w.w); err != nil {
			return err
		}
	case ArrayValueFunc:
		if _, err := w.w.Write([]byte("[")); err != nil {
			return err
		}

		if err := f(&elementWriter{w: w.w}); err != nil {
			return err
		}

		if _, err := w.w.Write([]byte("]")); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unexpected FuncType:%T", f))
	}

	return nil
}

type elementWriter struct {
	w         io.Writer
	following bool
}

func (ew *elementWriter) WriteElement(e interface{}) error {

	if ew.following {
		if _, err := ew.w.Write([]byte(",")); err != nil {
			return err
		}
	} else {
		ew.following = true
	}

	// Now Value in the e is not supported, and the key will directly marshalled.
	jsn, err := json.Marshal(e)
	if err != nil {
		return err
	}

	if _, err := ew.w.Write(jsn); err != nil {
		return err
	}

	return nil
}

// MarshalJSON implements json.Marshaler interface but it puts placeholder for delay encoding.
func (v *Value) MarshalJSON() ([]byte, error) {
	return json.Marshal(streamPrefix + v.key)
}
