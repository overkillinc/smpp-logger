package integration

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestIntegration_Connect(t *testing.T) {
	target := os.Getenv("SMPP_TEST_TARGET_HOST")
	if target == "" {
		target = "127.0.0.1:30075"
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", target)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", target, err)
	}
	defer conn.Close()

	// Basic write to ensure socket accepts data
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte{0x00}); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Best-effort read to avoid leaving connection half-open
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1)
	_, _ = conn.Read(buf)

	t.Logf("connected to %s OK", target)
}
