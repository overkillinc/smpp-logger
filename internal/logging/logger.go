package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
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

type Logger struct {
	format Format
	writer io.Writer
	json   *slog.Logger
	mu     sync.Mutex
}

func New(w io.Writer, format string) (*Logger, error) {
	switch Format(format) {
	case TextFormat:
		return &Logger{
			format: TextFormat,
			writer: w,
		}, nil
	case JSONFormat:
		return &Logger{
			format: JSONFormat,
			json: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})),
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

		l.mu.Lock()
		defer l.mu.Unlock()
		fmt.Fprintln(l.writer, strings.Join(parts, " "))
	}
}

func cleanString(value string) string {
	value = strings.ReplaceAll(value, "\r", "\\r")
	return strings.ReplaceAll(value, "\n", "\\n")
}
