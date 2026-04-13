package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutlv"
	"github.com/overkillinc/smpp-logger/internal/config"
	"github.com/overkillinc/smpp-logger/internal/logging"
)

const (
	statusInvalidMessageLength   pdu.Status = 0x00000001
	statusInvalidCommandID       pdu.Status = 0x00000003
	statusIncorrectBindStatus    pdu.Status = 0x00000004
	statusAlreadyBound           pdu.Status = 0x00000005
	statusInvalidDestinationAddr pdu.Status = 0x0000000B
	statusInvalidPassword        pdu.Status = 0x0000000E
	statusInvalidSystemID        pdu.Status = 0x0000000F
	statusInvalidParameterLength pdu.Status = 0x000000C2
)

type bindMode string

const (
	bindModeReceiver    bindMode = "receiver"
	bindModeTransmitter bindMode = "transmitter"
	bindModeTransceiver bindMode = "transceiver"
)

type Server struct {
	cfg    config.Config
	logger *logging.Logger

	now func() time.Time

	listenerMu sync.Mutex
	listener   net.Listener

	connsMu sync.Mutex
	conns   map[*conn]struct{}

	nextMessageID atomic.Uint64
	nextSequence  atomic.Uint32
}

type session struct {
	conn   *conn
	login  string
	mode   bindMode
	client string
}

type message struct {
	sender      string
	destination string
	text        string
	dataCoding  pdutext.DataCoding
}

func New(cfg config.Config, logger *logging.Logger) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
		conns:  make(map[*conn]struct{}),
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.ListenAddr, err)
	}
	return s.Serve(ctx, listener)
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	s.listenerMu.Lock()
	s.listener = listener
	s.listenerMu.Unlock()
	defer s.shutdown()

	go func() {
		<-ctx.Done()
		s.shutdown()
	}()

	for {
		netConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}

		clientConn := newConn(netConn)
		s.trackConn(clientConn)
		go s.handleConn(clientConn)
	}
}

func (s *Server) handleConn(clientConn *conn) {
	defer func() {
		s.untrackConn(clientConn)
		_ = clientConn.Close()
	}()

	sess, err := s.authenticate(clientConn)
	if err != nil {
		return
	}

	for {
		body, header, err := clientConn.Read()
		if err != nil {
			s.respondToReadError(clientConn, header, err)
			return
		}

		switch header.ID {
		case pdu.BindReceiverID, pdu.BindTransmitterID, pdu.BindTransceiverID:
			resp, _, ok := bindResponseFor(header.ID)
			if !ok {
				s.writeGenericNACK(clientConn, header.Seq, statusInvalidCommandID)
				continue
			}
			resp.Header().Seq = header.Seq
			resp.Header().Status = statusAlreadyBound
			_ = clientConn.Write(resp)
		case pdu.EnquireLinkID:
			resp := pdu.NewEnquireLinkRespSeq(header.Seq)
			_ = clientConn.Write(resp)
		case pdu.SubmitSMID:
			if err := s.handleSubmit(sess, body); err != nil {
				return
			}
		case pdu.DeliverSMRespID, pdu.EnquireLinkRespID, pdu.SubmitSMRespID:
			continue
		case pdu.UnbindID:
			resp := pdu.NewUnbindResp()
			resp.Header().Seq = header.Seq
			_ = clientConn.Write(resp)
			s.logger.LogUnbind(sess.login, sess.client)
			return
		default:
			s.writeGenericNACK(clientConn, header.Seq, statusInvalidCommandID)
		}
	}
}

func (s *Server) authenticate(clientConn *conn) (*session, error) {
	body, header, err := clientConn.Read()
	if err != nil {
		s.respondToReadError(clientConn, header, err)
		return nil, err
	}

	resp, mode, ok := bindResponseFor(header.ID)
	if !ok {
		s.writeGenericNACK(clientConn, header.Seq, statusIncorrectBindStatus)
		return nil, fmt.Errorf("first pdu %s is not a bind request", header.ID)
	}

	resp.Header().Seq = header.Seq

	login := fieldString(body.Fields(), pdufield.SystemID)
	password := fieldString(body.Fields(), pdufield.Password)

	switch {
	case login == "":
		resp.Header().Status = statusInvalidSystemID
	case password == "":
		resp.Header().Status = statusInvalidPassword
	case login != s.cfg.SystemID:
		resp.Header().Status = statusInvalidSystemID
	case password != s.cfg.Password:
		resp.Header().Status = statusInvalidPassword
	default:
		resp.Fields().Set(pdufield.SystemID, s.cfg.SystemID)
	}

	if err := clientConn.Write(resp); err != nil {
		return nil, err
	}
	if resp.Header().Status != 0 {
		return nil, fmt.Errorf("bind rejected for %s: %s", login, resp.Header().Status.Error())
	}

	sess := &session{
		conn:   clientConn,
		login:  login,
		mode:   mode,
		client: clientConn.RemoteAddr().String(),
	}
	s.logger.LogBind(sess.login, string(sess.mode), sess.client)
	return sess, nil
}

func (s *Server) handleSubmit(sess *session, body pdu.Body) error {
	resp := pdu.NewSubmitSMResp()
	resp.Header().Seq = body.Header().Seq

	if !sess.mode.canSubmit() {
		resp.Header().Status = statusIncorrectBindStatus
		return sess.conn.Write(resp)
	}

	msg, status := decodeMessage(body)
	if status != 0 {
		resp.Header().Status = status
		return sess.conn.Write(resp)
	}

	messageID := s.newMessageID()
	if err := resp.Fields().Set(pdufield.MessageID, messageID); err != nil {
		return err
	}
	if err := sess.conn.Write(resp); err != nil {
		return err
	}

	s.logger.LogSubmit(logging.Event{
		Login:       sess.login,
		Sender:      msg.sender,
		Destination: msg.destination,
		Text:        msg.text,
		MessageID:   messageID,
		ClientAddr:  sess.client,
		Sequence:    body.Header().Seq,
	})

	if !sess.mode.canReceive() {
		return nil
	}

	receipt, err := s.buildReceipt(body, msg, messageID)
	if err != nil {
		return err
	}
	if err := sess.conn.Write(receipt); err != nil {
		return err
	}

	s.logger.LogReceipt(logging.Event{
		Login:       sess.login,
		Sender:      msg.destination,
		Destination: msg.sender,
		Text:        msg.text,
		MessageID:   messageID,
		ClientAddr:  sess.client,
		Sequence:    receipt.Header().Seq,
	})

	return nil
}

func (s *Server) buildReceipt(submit pdu.Body, msg message, messageID string) (pdu.Body, error) {
	receipt := pdu.NewDeliverSM()
	receipt.Header().Seq = s.newSequence()

	fields := receipt.Fields()
	for name, value := range map[pdufield.Name]interface{}{
		pdufield.ServiceType:          fieldString(submit.Fields(), pdufield.ServiceType),
		pdufield.SourceAddrTON:        uint8(0),
		pdufield.SourceAddrNPI:        uint8(0),
		pdufield.SourceAddr:           msg.destination,
		pdufield.DestAddrTON:          uint8(0),
		pdufield.DestAddrNPI:          uint8(0),
		pdufield.DestinationAddr:      msg.sender,
		pdufield.ESMClass:             uint8(0x04),
		pdufield.ProtocolID:           uint8(0),
		pdufield.PriorityFlag:         uint8(0),
		pdufield.ScheduleDeliveryTime: "",
		pdufield.ValidityPeriod:       "",
		pdufield.RegisteredDelivery:   uint8(pdufield.NoDeliveryReceipt),
		pdufield.ReplaceIfPresentFlag: uint8(0),
		pdufield.DataCoding:           uint8(pdutext.DefaultType),
		pdufield.SMDefaultMsgID:       uint8(0),
		pdufield.ShortMessage:         pdutext.Raw(formatReceiptText(s.now().UTC(), messageID, msg.text)),
	} {
		if err := fields.Set(name, value); err != nil {
			return nil, err
		}
	}

	return receipt, nil
}

func decodeMessage(body pdu.Body) (message, pdu.Status) {
	fields := body.Fields()
	msg := message{
		sender:      fieldString(fields, pdufield.SourceAddr),
		destination: fieldString(fields, pdufield.DestinationAddr),
	}

	if msg.destination == "" {
		return message{}, statusInvalidDestinationAddr
	}

	msg.dataCoding = readDataCoding(fields)
	payload := shortMessageBytes(body)
	msg.text = decodeMessageText(payload, msg.dataCoding)

	return msg, 0
}

func shortMessageBytes(body pdu.Body) []byte {
	if value, ok := body.Fields()[pdufield.ShortMessage]; ok {
		if raw, ok := value.Raw().([]byte); ok && len(raw) > 0 {
			return raw
		}
	}

	if value, ok := body.TLVFields()[pdutlv.TagMessagePayload]; ok {
		return value.Bytes()
	}

	return nil
}

func decodeMessageText(raw []byte, dataCoding pdutext.DataCoding) string {
	switch dataCoding {
	case pdutext.Latin1Type:
		return string(pdutext.Latin1(raw).Decode())
	case pdutext.ISO88595Type:
		return string(pdutext.ISO88595(raw).Decode())
	case pdutext.UCS2Type:
		return string(pdutext.UCS2(raw).Decode())
	default:
		return string(pdutext.Raw(raw).Decode())
	}
}

func readDataCoding(fields pdufield.Map) pdutext.DataCoding {
	value, ok := fields[pdufield.DataCoding]
	if !ok {
		return pdutext.DefaultType
	}

	raw := value.Raw()
	switch typed := raw.(type) {
	case uint8:
		return pdutext.DataCoding(typed)
	case []byte:
		if len(typed) > 0 {
			return pdutext.DataCoding(typed[0])
		}
	}

	return pdutext.DefaultType
}

func fieldString(fields pdufield.Map, name pdufield.Name) string {
	field, ok := fields[name]
	if !ok {
		return ""
	}
	return strings.TrimSuffix(field.String(), "\x00")
}

func bindResponseFor(id pdu.ID) (pdu.Body, bindMode, bool) {
	switch id {
	case pdu.BindReceiverID:
		return pdu.NewBindReceiverResp(), bindModeReceiver, true
	case pdu.BindTransmitterID:
		return pdu.NewBindTransmitterResp(), bindModeTransmitter, true
	case pdu.BindTransceiverID:
		return pdu.NewBindTransceiverResp(), bindModeTransceiver, true
	default:
		return nil, "", false
	}
}

func formatReceiptText(now time.Time, messageID, text string) string {
	timestamp := now.Format("0601021504")
	return fmt.Sprintf(
		"id:%s sub:001 dlvrd:001 submit date:%s done date:%s stat:DELIVRD err:000 text:%s",
		messageID,
		timestamp,
		timestamp,
		receiptSnippet(text),
	)
}

func receiptSnippet(text string) string {
	runes := []rune(text)
	if len(runes) <= 20 {
		return text
	}
	return string(runes[:20])
}

func (m bindMode) canSubmit() bool {
	return m == bindModeTransmitter || m == bindModeTransceiver
}

func (m bindMode) canReceive() bool {
	return m == bindModeReceiver || m == bindModeTransceiver
}

func (s *Server) newMessageID() string {
	return fmt.Sprintf("msg-%010d", s.nextMessageID.Add(1))
}

func (s *Server) newSequence() uint32 {
	return s.nextSequence.Add(1)
}

func (s *Server) respondToReadError(clientConn *conn, header *pdu.Header, err error) {
	if header == nil {
		return
	}
	status := statusInvalidParameterLength
	switch {
	case errors.Is(err, errInvalidMessageLength):
		status = statusInvalidMessageLength
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		status = statusInvalidParameterLength
	}
	s.writeGenericNACK(clientConn, header.Seq, status)
}

func (s *Server) writeGenericNACK(clientConn *conn, seq uint32, status pdu.Status) {
	resp := pdu.NewGenericNACK()
	resp.Header().Seq = seq
	resp.Header().Status = status
	_ = clientConn.Write(resp)
}

func (s *Server) trackConn(clientConn *conn) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	s.conns[clientConn] = struct{}{}
}

func (s *Server) untrackConn(clientConn *conn) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	delete(s.conns, clientConn)
}

func (s *Server) shutdown() {
	s.listenerMu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	s.listenerMu.Unlock()

	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for clientConn := range s.conns {
		_ = clientConn.Close()
		delete(s.conns, clientConn)
	}
}
