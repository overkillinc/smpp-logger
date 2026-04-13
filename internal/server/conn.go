package server

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/fiorix/go-smpp/smpp/pdu"
)

var errInvalidMessageLength = errors.New("invalid message length")

type conn struct {
	netConn net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	mu      sync.Mutex
}

func newConn(netConn net.Conn) *conn {
	return &conn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		writer:  bufio.NewWriter(netConn),
	}
}

func (c *conn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}

func (c *conn) Read() (pdu.Body, *pdu.Header, error) {
	header, err := pdu.DecodeHeader(c.reader)
	if err != nil {
		return nil, nil, err
	}

	if header.Len < pdu.HeaderLen || header.Len > pdu.MaxSize {
		return nil, header, errInvalidMessageLength
	}

	payloadLength := int(header.Len) - pdu.HeaderLen
	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, header, err
	}

	var raw bytes.Buffer
	if err := header.SerializeTo(&raw); err != nil {
		return nil, header, err
	}
	if _, err := raw.Write(payload); err != nil {
		return nil, header, err
	}

	body, err := pdu.Decode(&raw)
	if err != nil {
		return nil, header, err
	}

	return body, header, nil
}

func (c *conn) Write(body pdu.Body) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var raw bytes.Buffer
	if err := body.SerializeTo(&raw); err != nil {
		return err
	}
	if _, err := io.Copy(c.writer, &raw); err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *conn) Close() error {
	return c.netConn.Close()
}
