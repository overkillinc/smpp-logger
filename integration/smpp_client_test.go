package integration

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"testing"
	"time"

	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutlv"
)

func TestIntegration_SmppFlow(t *testing.T) {
	// Run integration tests only when explicitly enabled to avoid masking real failures.
	// Priority:
	// 1) If SMPP_TEST_TARGET_HOST is set -> use it and run
	// 2) Else, require RUN_INTEGRATION=true to run against localhost:30075 (default local NodePort)
	target := os.Getenv("SMPP_TEST_TARGET_HOST")
	if target == "" {
		if os.Getenv("RUN_INTEGRATION") != "true" {
			t.Skip("integration tests disabled; set RUN_INTEGRATION=true to run local integration tests, or set SMPP_TEST_TARGET_HOST to point to a remote server")
		}
		// default local target when RUN_INTEGRATION is enabled
		target = "127.0.0.1:30075"
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	c, err := dialer.Dial("tcp", target)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer c.Close()

	reader := bufio.NewReader(c)
	writer := bufio.NewWriter(c)

	// Bind transceiver
	bind := pdu.NewBindTransceiver()
	bind.Header().Seq = 1
	sysid := os.Getenv("SMPP_TEST_SYSTEM_ID")
	if sysid == "" {
		sysid = "manzana"
	}
	pw := os.Getenv("SMPP_TEST_PASSWORD")
	if pw == "" {
		pw = "039e263cff6ad8fe"
	}
	_ = bind.Fields().Set(pdufield.SystemID, sysid)
	_ = bind.Fields().Set(pdufield.Password, pw)
	if err := bind.SerializeTo(writer); err != nil {
		t.Fatalf("serialize bind: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush bind: %v", err)
	}

	// Read bind resp
	head, err := pdu.DecodeHeader(reader)
	if err != nil {
		t.Fatalf("read bind header: %v", err)
	}
	payload := make([]byte, int(head.Len)-pdu.HeaderLen)
	if _, err := reader.Read(payload); err != nil {
		t.Fatalf("read bind payload: %v", err)
	}
	var raw bytes.Buffer
	if err := head.SerializeTo(&raw); err != nil {
		t.Fatalf("serialize header to raw: %v", err)
	}
	if _, err := raw.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	b, err := pdu.Decode(&raw)
	if err != nil {
		t.Fatalf("decode bind resp: %v", err)
	}
	if b.Header().Status != 0 {
		t.Fatalf("bind failed status=%v", b.Header().Status)
	}

	// submit_sm
	submit := pdu.NewSubmitSM(pdutlv.Fields{})
	submit.Header().Seq = 2
	_ = submit.Fields().Set(pdufield.SourceAddr, "alice")
	_ = submit.Fields().Set(pdufield.DestinationAddr, "15551234567")
	_ = submit.Fields().Set(pdufield.ShortMessage, pdutext.Raw("hello k8s"))
	_ = submit.Fields().Set(pdufield.RegisteredDelivery, uint8(pdufield.FinalDeliveryReceipt))
	if err := submit.SerializeTo(writer); err != nil {
		t.Fatalf("serialize submit: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush submit: %v", err)
	}

	// read submit_resp
	head, err = pdu.DecodeHeader(reader)
	if err != nil {
		t.Fatalf("read submit_resp header: %v", err)
	}
	payload = make([]byte, int(head.Len)-pdu.HeaderLen)
	if _, err := reader.Read(payload); err != nil {
		t.Fatalf("read submit_resp payload: %v", err)
	}
	raw.Reset()
	if err := head.SerializeTo(&raw); err != nil {
		t.Fatalf("serialize header: %v", err)
	}
	if _, err := raw.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	resp, err := pdu.Decode(&raw)
	if err != nil {
		t.Fatalf("decode submit_resp: %v", err)
	}
	if resp.Header().Status != 0 {
		t.Fatalf("submit_resp status=%v", resp.Header().Status)
	}

	// read deliver_sm (receipt)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	head, err = pdu.DecodeHeader(reader)
	if err != nil {
		t.Fatalf("read deliver header: %v", err)
	}
	payload = make([]byte, int(head.Len)-pdu.HeaderLen)
	if _, err := reader.Read(payload); err != nil {
		t.Fatalf("read deliver payload: %v", err)
	}
	raw.Reset()
	if err := head.SerializeTo(&raw); err != nil {
		t.Fatalf("serialize head: %v", err)
	}
	if _, err := raw.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	recv, err := pdu.Decode(&raw)
	if err != nil {
		t.Fatalf("decode deliver: %v", err)
	}
	if recv.Header().ID != pdu.DeliverSMID {
		t.Fatalf("unexpected pdu id: %v", recv.Header().ID)
	}

	// send deliver_sm_resp
	respP := pdu.NewDeliverSMRespSeq(recv.Header().Seq)
	respP.Header().Seq = 5
	if err := respP.SerializeTo(writer); err != nil {
		t.Fatalf("serialize deliver_resp: %v", err)
	}
	_ = writer.Flush()

	// unbind
	unbind := pdu.NewUnbind()
	unbind.Header().Seq = 6
	if err := unbind.SerializeTo(writer); err != nil {
		t.Fatalf("serialize unbind: %v", err)
	}
	_ = writer.Flush()
}
