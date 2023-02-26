package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Color int

var (
	ColorBlack   = Color(0)
	ColorRed     = Color(1)
	ColorGreen   = Color(2)
	ColorYellow  = Color(3)
	ColorBlue    = Color(4)
	ColorMagenta = Color(5)
	ColorCyan    = Color(6)
	ColorWhite   = Color(7)
)

type TerminalBackendCfg struct {
	Color       bool `json:"color"`
	DomainWidth int  `json:"domain_width"`
}

type TerminalBackend struct {
	Cfg TerminalBackendCfg

	domainWidth int
}

func NewTerminalBackend(cfg TerminalBackendCfg) *TerminalBackend {
	domainWidth := 24
	if cfg.DomainWidth > 0 {
		domainWidth = cfg.DomainWidth
	}

	isCharDev, err := isCharDevice(os.Stderr)
	if err != nil {
		// If we cannot check for some reason, assume it is a character device
		isCharDev = true
	}

	if !isCharDev {
		cfg.Color = false
	}

	b := &TerminalBackend{
		Cfg: cfg,

		domainWidth: domainWidth,
	}

	return b
}

func (b *TerminalBackend) Log(msg Message) {
	var buf bytes.Buffer

	domain := fmt.Sprintf("%-*s", b.domainWidth, msg.domain)

	fmt.Fprintf(&buf, "%-7s  %s  %s\n",
		msg.Level, b.colorize(ColorGreen, domain), msg.Message)

	if len(msg.Data) > 0 {
		fmt.Fprintf(&buf, "         ")

		keys := make([]string, len(msg.Data))
		i := 0
		for k := range msg.Data {
			keys[i] = k
			i++
		}
		sort.Strings(keys)

		for i, k := range keys {
			if i > 0 {
				fmt.Fprintf(&buf, " ")
			}

			fmt.Fprintf(&buf, "%s=%s",
				b.colorize(ColorBlue, k), formatDatum(msg.Data[k]))

			i++
		}

		fmt.Fprintf(&buf, "\n")
	}

	io.Copy(os.Stderr, &buf)
}

func (b *TerminalBackend) colorize(color Color, s string) string {
	if !b.Cfg.Color {
		return s
	}

	return fmt.Sprintf("\033[%dm%s\033[0m", 30+int(color), s)
}

func formatDatum(datum Datum) string {
	switch v := datum.(type) {
	case fmt.Stringer:
		return formatDatum(v.String())

	case string:
		if !strings.Contains(v, " ") {
			return v
		}

		return fmt.Sprintf("%q", v)

	default:
		return fmt.Sprintf("%v", v)
	}
}

func isCharDevice(file *os.File) (bool, error) {
	info, err := file.Stat()
	if err != nil {
		return false, err
	}

	return info.Mode()&os.ModeCharDevice != 0, nil
}
