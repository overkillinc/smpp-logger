package integration

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestIntegration_Connect(t *testing.T) {
	// Run integration tests only when explicitly enabled. Use SMPP_TEST_TARGET_HOST for remote targets
	// or set RUN_INTEGRATION=true to test against localhost:30075.
	target := os.Getenv("SMPP_TEST_TARGET_HOST")
	if target == "" {
		if os.Getenv("RUN_INTEGRATION") != "true" {
			t.Skip("integration tests disabled; set RUN_INTEGRATION=true to run local integration tests, or set SMPP_TEST_TARGET_HOST to point to a remote server")
		}
		// default local target when RUN_INTEGRATION is enabled
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
