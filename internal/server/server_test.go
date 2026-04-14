package server

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutlv"
	"github.com/overkillinc/smpp-logger/internal/config"
	"github.com/overkillinc/smpp-logger/internal/logging"
)

func TestServerSubmitFlowLogsAndReceipt(t *testing.T) {
	cfg, logs, addr := startTestServer(t)

	client := dialTestClient(t, addr)
	defer client.Close()

	writePDU(t, client, bindPDU(t, pdu.NewBindTransceiver(), cfg.SystemID, cfg.Password, 1))
	bindResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read bind response: %v", err)
	}
	if bindResp.Header().Status != 0 {
		t.Fatalf("bind status = %v", bindResp.Header().Status)
	}

	enquire := pdu.NewEnquireLink()
	enquire.Header().Seq = 2
	writePDU(t, client, enquire)
	enquireResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read enquire_link_resp: %v", err)
	}
	if enquireResp.Header().ID != pdu.EnquireLinkRespID {
		t.Fatalf("response ID = %v, want enquire_link_resp", enquireResp.Header().ID)
	}

	submit := pdu.NewSubmitSM(pdutlv.Fields{})
	submit.Header().Seq = 3
	mustSetField(t, submit.Fields(), pdufield.SourceAddr, "alice")
	mustSetField(t, submit.Fields(), pdufield.DestinationAddr, "15551234567")
	mustSetField(t, submit.Fields(), pdufield.ShortMessage, pdutext.Raw("hello world"))
	mustSetField(t, submit.Fields(), pdufield.RegisteredDelivery, uint8(pdufield.FinalDeliveryReceipt))
	writePDU(t, client, submit)

	submitResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read submit_sm_resp: %v", err)
	}
	if submitResp.Header().ID != pdu.SubmitSMRespID {
		t.Fatalf("response ID = %v, want submit_sm_resp", submitResp.Header().ID)
	}
	if submitResp.Header().Status != 0 {
		t.Fatalf("submit status = %v", submitResp.Header().Status)
	}

	messageID := fieldString(submitResp.Fields(), pdufield.MessageID)
	if messageID == "" {
		t.Fatal("submit_sm_resp missing message id")
	}

	receipt, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read deliver_sm: %v", err)
	}
	if receipt.Header().ID != pdu.DeliverSMID {
		t.Fatalf("response ID = %v, want deliver_sm", receipt.Header().ID)
	}
	if got := fieldString(receipt.Fields(), pdufield.SourceAddr); got != "15551234567" {
		t.Fatalf("deliver_sm source_addr = %q", got)
	}
	if got := fieldString(receipt.Fields(), pdufield.DestinationAddr); got != "alice" {
		t.Fatalf("deliver_sm destination_addr = %q", got)
	}
	if got := fieldString(receipt.Fields(), pdufield.ShortMessage); !strings.Contains(got, "stat:DELIVRD") || !strings.Contains(got, "id:"+messageID) {
		t.Fatalf("deliver_sm short_message = %q", got)
	}

	writePDU(t, client, pdu.NewDeliverSMRespSeq(receipt.Header().Seq))

	unbind := pdu.NewUnbind()
	unbind.Header().Seq = 4
	writePDU(t, client, unbind)
	unbindResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read unbind_resp: %v", err)
	}
	if unbindResp.Header().ID != pdu.UnbindRespID {
		t.Fatalf("response ID = %v, want unbind_resp", unbindResp.Header().ID)
	}

	// logs may be written asynchronously; retry briefly to avoid flaky failures
	wanted := []string{
		`event=bind`,
		`event=submit`,
		`event=receipt`,
		`event=unbind`,
		`login="` + cfg.SystemID + `"`,
		`sender="alice"`,
		`destination="15551234567"`,
		`text="hello world"`,
		`message_id="` + messageID + `"`,
	}
	var output string
	foundAll := false
	for i := 0; i < 50; i++ { // up to ~500ms
		output = logs.String()
		missing := false
		for _, want := range wanted {
			if !strings.Contains(output, want) {
				missing = true
				break
			}
		}
		if !missing {
			foundAll = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !foundAll {
		t.Fatalf("log output missing items after wait: %s", output)
	}
}

func TestServerRejectsInvalidPassword(t *testing.T) {
	cfg, _, addr := startTestServer(t)

	client := dialTestClient(t, addr)
	defer client.Close()

	writePDU(t, client, bindPDU(t, pdu.NewBindTransceiver(), cfg.SystemID, "wrong", 1))
	resp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read bind response: %v", err)
	}
	if resp.Header().Status != statusInvalidPassword {
		t.Fatalf("bind status = %v, want %v", resp.Header().Status, statusInvalidPassword)
	}
}

func TestServerRejectsUnexpectedFirstPDU(t *testing.T) {
	_, _, addr := startTestServer(t)

	client := dialTestClient(t, addr)
	defer client.Close()

	enquire := pdu.NewEnquireLink()
	enquire.Header().Seq = 1
	writePDU(t, client, enquire)

	resp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read generic_nack: %v", err)
	}
	if resp.Header().ID != pdu.GenericNACKID {
		t.Fatalf("response ID = %v, want generic_nack", resp.Header().ID)
	}
	if resp.Header().Status != statusIncorrectBindStatus {
		t.Fatalf("generic_nack status = %v, want %v", resp.Header().Status, statusIncorrectBindStatus)
	}
}

func TestServerRejectsSubmitOnReceiverBind(t *testing.T) {
	cfg, _, addr := startTestServer(t)

	client := dialTestClient(t, addr)
	defer client.Close()

	writePDU(t, client, bindPDU(t, pdu.NewBindReceiver(), cfg.SystemID, cfg.Password, 1))
	resp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read bind response: %v", err)
	}
	if resp.Header().Status != 0 {
		t.Fatalf("bind status = %v", resp.Header().Status)
	}

	submit := pdu.NewSubmitSM(pdutlv.Fields{})
	submit.Header().Seq = 2
	mustSetField(t, submit.Fields(), pdufield.SourceAddr, "alice")
	mustSetField(t, submit.Fields(), pdufield.DestinationAddr, "15551234567")
	mustSetField(t, submit.Fields(), pdufield.ShortMessage, pdutext.Raw("hello world"))
	writePDU(t, client, submit)

	submitResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read submit_sm_resp: %v", err)
	}
	if submitResp.Header().Status != statusIncorrectBindStatus {
		t.Fatalf("submit status = %v, want %v", submitResp.Header().Status, statusIncorrectBindStatus)
	}
}

func TestServerRejectsSubmitWithoutDestination(t *testing.T) {
	cfg, _, addr := startTestServer(t)

	client := dialTestClient(t, addr)
	defer client.Close()

	writePDU(t, client, bindPDU(t, pdu.NewBindTransceiver(), cfg.SystemID, cfg.Password, 1))
	resp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read bind response: %v", err)
	}
	if resp.Header().Status != 0 {
		t.Fatalf("bind status = %v", resp.Header().Status)
	}

	submit := pdu.NewSubmitSM(pdutlv.Fields{})
	submit.Header().Seq = 2
	mustSetField(t, submit.Fields(), pdufield.SourceAddr, "alice")
	mustSetField(t, submit.Fields(), pdufield.ShortMessage, pdutext.Raw("hello world"))
	writePDU(t, client, submit)

	submitResp, _, err := client.Read()
	if err != nil {
		t.Fatalf("Read submit_sm_resp: %v", err)
	}
	if submitResp.Header().Status != statusInvalidDestinationAddr {
		t.Fatalf("submit status = %v, want %v", submitResp.Header().Status, statusInvalidDestinationAddr)
	}
}

func TestDecodeMessageText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        []byte
		dataCoding pdutext.DataCoding
		want       string
	}{
		{
			name:       "raw",
			raw:        []byte("hello"),
			dataCoding: pdutext.DefaultType,
			want:       "hello",
		},
		{
			name:       "latin1",
			raw:        pdutext.Latin1([]byte("ol\xe1")).Encode(),
			dataCoding: pdutext.Latin1Type,
			want:       "olá",
		},
		{
			name:       "ucs2",
			raw:        pdutext.UCS2([]byte("hi ✓")).Encode(),
			dataCoding: pdutext.UCS2Type,
			want:       "hi ✓",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := decodeMessageText(tt.raw, tt.dataCoding); got != tt.want {
				t.Fatalf("decodeMessageText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func startTestServer(t *testing.T) (config.Config, *bytes.Buffer, string) {
	t.Helper()

	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
		SystemID:        "client",
		Password:        "secret",
		LogFormat:       "text",
		ShutdownTimeout: 3 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	logs := &bytes.Buffer{}
	logger, err := logging.New(logs, cfg.LogFormat)
	if err != nil {
		t.Fatalf("logging.New: %v", err)
	}

	srv := New(cfg, logger)
	srv.now = func() time.Time {
		return time.Date(2026, time.April, 13, 16, 0, 0, 0, time.UTC)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx, listener)
	}()

	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	})

	return cfg, logs, listener.Addr().String()
}

func dialTestClient(t *testing.T, addr string) *conn {
	t.Helper()

	netConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return newConn(netConn)
}

func bindPDU(t *testing.T, body pdu.Body, systemID, password string, seq uint32) pdu.Body {
	t.Helper()

	body.Header().Seq = seq
	mustSetField(t, body.Fields(), pdufield.SystemID, systemID)
	mustSetField(t, body.Fields(), pdufield.Password, password)
	return body
}

func writePDU(t *testing.T, client *conn, body pdu.Body) {
	t.Helper()
	if err := client.Write(body); err != nil {
		t.Fatalf("Write PDU: %v", err)
	}
}

func mustSetField(t *testing.T, fields pdufield.Map, name pdufield.Name, value interface{}) {
	t.Helper()
	if err := fields.Set(name, value); err != nil {
		t.Fatalf("Set field %s: %v", name, err)
	}
}
