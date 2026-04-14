package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Format string

const (
	TextFormat Format = "text"
	JSONFormat Format = "json"
)

type Event struct {
	Login       string
	Mode        string
	Sender      string
	Destination string
	Text        string
	MessageID   string
	ClientAddr  string
	Sequence    uint32
}

type LogEntry struct {
	Time time.Time
	Line string
}

type Logger struct {
	format Format
	writer io.Writer
	json   *slog.Logger
	mu     sync.Mutex

	// in-memory history of recent log lines for the HTTP UI
	history    []LogEntry
	historyMu  sync.RWMutex
	historyMax int
}

func New(w io.Writer, format string) (*Logger, error) {
	const defaultHistory = 10000
	switch Format(format) {
	case TextFormat:
		return &Logger{
			format:     TextFormat,
			writer:     w,
			historyMax: defaultHistory,
		}, nil
	case JSONFormat:
		return &Logger{
			format: JSONFormat,
			json: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})),
			historyMax: defaultHistory,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func (l *Logger) LogBind(login, mode, clientAddr string) {
	l.log("bind", Event{
		Login:      login,
		Mode:       mode,
		ClientAddr: clientAddr,
	})
}

func (l *Logger) LogUnbind(login, clientAddr string) {
	l.log("unbind", Event{
		Login:      login,
		ClientAddr: clientAddr,
	})
}

func (l *Logger) LogSubmit(event Event) {
	l.log("submit", event)
}

func (l *Logger) LogReceipt(event Event) {
	l.log("receipt", event)
}

func (l *Logger) log(kind string, event Event) {
	if l == nil {
		return
	}

	// Build a canonical text line for history regardless of current output format
	parts := []string{"event=" + kind}
	appendString := func(key, value string) {
		if value == "" {
			return
		}
		parts = append(parts, key+"="+strconv.Quote(cleanString(value)))
	}
	appendString("login", event.Login)
	appendString("mode", event.Mode)
	appendString("sender", event.Sender)
	appendString("destination", event.Destination)
	appendString("text", event.Text)
	appendString("message_id", event.MessageID)
	appendString("client", event.ClientAddr)
	if event.Sequence != 0 {
		parts = append(parts, fmt.Sprintf("seq=%d", event.Sequence))
	}
	line := strings.Join(parts, " ")

	switch l.format {
	case JSONFormat:
		l.json.Info("smpp event",
			slog.String("event", kind),
			slog.String("login", event.Login),
			slog.String("mode", event.Mode),
			slog.String("sender", event.Sender),
			slog.String("destination", event.Destination),
			slog.String("text", event.Text),
			slog.String("message_id", event.MessageID),
			slog.String("client_addr", event.ClientAddr),
			slog.Uint64("sequence", uint64(event.Sequence)),
		)
	default:
		l.mu.Lock()
		fmt.Fprintln(l.writer, line)
		l.mu.Unlock()
	}

	// Append to in-memory history
	if l.historyMax > 0 {
		l.historyMu.Lock()
		defer l.historyMu.Unlock()
		l.history = append(l.history, LogEntry{Time: time.Now().UTC(), Line: line})
		if len(l.history) > l.historyMax {
			// drop oldest
			l.history = l.history[len(l.history)-l.historyMax:]
		}
	}
}

func cleanString(value string) string {
	value = strings.ReplaceAll(value, "\r", "\\r")
	return strings.ReplaceAll(value, "\n", "\\n")
}

// Recent returns log lines written in the last 'minutes' minutes in chronological order.
func (l *Logger) Recent(minutes int) []string {
	if l == nil || minutes <= 0 {
		return nil
	}
	cut := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	l.historyMu.RLock()
	defer l.historyMu.RUnlock()

	var out []string
	for _, e := range l.history {
		if e.Time.After(cut) || e.Time.Equal(cut) {
			out = append(out, e.Line)
		}
	}
	return out
}
