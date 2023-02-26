package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type LoggerCfg struct {
	BackendType BackendType      `json:"backend_type"`
	BackendData *json.RawMessage `json:"backend,omitempty"`
	Backend     interface{}      `json:"-"`
}

type Logger struct {
	Cfg     LoggerCfg
	Backend Backend
	Domain  string
	Data    Data
}

func DefaultLogger(name string) *Logger {
	backendCfg := TerminalBackendCfg{
		Color: true,
	}

	backend := NewTerminalBackend(backendCfg)

	return &Logger{
		Cfg:     LoggerCfg{},
		Backend: backend,
		Domain:  name,
		Data:    Data{},
	}
}

func NewLogger(name string, cfg LoggerCfg) (*Logger, error) {
	l := &Logger{
		Cfg: cfg,

		Domain: name,
		Data:   Data{},
	}

	backendCfg := func(cfgObj interface{}) (interface{}, error) {
		switch {
		case cfg.Backend != nil:
			return cfg.Backend, nil

		case cfg.BackendData != nil:
			if err := json.Unmarshal(*cfg.BackendData, cfgObj); err != nil {
				return nil,
					fmt.Errorf("invalid backend configuration: %w", err)
			}
		}

		return cfgObj, nil
	}

	switch cfg.BackendType {
	case BackendTypeTerminal:
		bcfg, err := backendCfg(&TerminalBackendCfg{})
		if err != nil {
			return nil, err
		}
		bcfg2 := bcfg.(*TerminalBackendCfg)
		l.Backend = NewTerminalBackend(*bcfg2)

	case "":
		return nil, fmt.Errorf("missing or empty backend type")

	default:
		return nil, fmt.Errorf("invalid backend type %q", cfg.BackendType)
	}

	return l, nil
}

func (l *Logger) Child(domain string, data Data) *Logger {
	childDomain := l.Domain
	if domain != "" {
		childDomain += "." + domain
	}

	child := &Logger{
		Cfg:     l.Cfg,
		Backend: l.Backend,

		Domain: childDomain,
		Data:   MergeData(l.Data, data),
	}

	return child
}

func (l *Logger) Log(msg Message) {
	var t time.Time
	if msg.Time == nil {
		t = time.Now()
	} else {
		t = *msg.Time
	}

	t = t.UTC()
	msg.Time = &t

	msg.domain = l.Domain

	if msg.Data == nil {
		msg.Data = make(Data)
	}

	msg.Data = MergeData(l.Data, msg.Data)

	l.Backend.Log(msg)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(Message{
		Level:   LevelInfo,
		Message: fmt.Sprintf(format, args...),
	})
}

func (l *Logger) InfoData(data Data, format string, args ...interface{}) {
	l.Log(Message{
		Level:   LevelInfo,
		Message: fmt.Sprintf(format, args...),
		Data:    data,
	})
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(Message{
		Level:   LevelError,
		Message: fmt.Sprintf(format, args...),
	})
}

func (l *Logger) ErrorData(data Data, format string, args ...interface{}) {
	l.Log(Message{
		Level:   LevelError,
		Message: fmt.Sprintf(format, args...),
		Data:    data,
	})
}

func (l *Logger) StdLogger(level Level) *log.Logger {
	// The standard log package does not support log levels, so we have to
	// choose one to be used for all messages.
	//
	// Standard loggers use the io.Writer interface as sink, which does not
	// allow any parameter. We pass the level at the beginning of the message
	// followed by an ASCII unit separator.
	return log.New(l, string(level)+"\x1f", 0)
}

func (l *Logger) Write(data []byte) (int, error) {
	level := LevelInfo
	var msg string

	idx := bytes.IndexByte(data, 0x1f)
	if idx >= 0 {
		isKnownLevel := true

		levelString := string(data[:idx])
		switch levelString {
		case "info":
			level = LevelInfo
		case "error":
			level = LevelError
		default:
			isKnownLevel = false
		}

		if isKnownLevel {
			msg = string(data[idx+1:])
		} else {
			msg = string(data)
		}
	}

	msg = strings.TrimSpace(msg)

	l.Log(Message{
		Level:   level,
		Message: msg,
	})

	return len(data), nil
}
