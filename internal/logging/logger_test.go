package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestTextLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := New(&buf, string(TextFormat))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.LogSubmit(Event{
		Login:       "client",
		Sender:      "alice",
		Destination: "12345",
		Text:        "hello\nworld",
		MessageID:   "msg-1",
		ClientAddr:  "127.0.0.1:1234",
		Sequence:    9,
	})

	output := buf.String()
	for _, want := range []string{
		`event=submit`,
		`login="client"`,
		`sender="alice"`,
		`destination="12345"`,
		`text="hello\\nworld"`,
		`message_id="msg-1"`,
		`client="127.0.0.1:1234"`,
		`seq=9`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestJSONLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := New(&buf, string(JSONFormat))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.LogBind("client", "transceiver", "127.0.0.1:1234")

	output := buf.String()
	for _, want := range []string{
		`"event":"bind"`,
		`"login":"client"`,
		`"mode":"transceiver"`,
		`"client_addr":"127.0.0.1:1234"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}
