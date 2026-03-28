package dashboard

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Minimal WebSocket implementation (RFC 6455) — no external dependencies.
// Supports text frames only, which is sufficient for JSON event streaming.

const websocketGUID = "258EAFA5-E914-47DA-95CA-5AB5DC525C63"

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		r.Header.Get("Sec-WebSocket-Key") != ""
}

func acceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsConn wraps a hijacked connection with separate buffered reader/writer
// and a mutex protecting all writes.
type wsConn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	wmu    sync.Mutex // protects all writes
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket upgrade not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijack not supported")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	// Flush any pre-buffered data from the HTTP server before writing our handshake
	if bufrw.Writer.Buffered() > 0 {
		bufrw.Writer.Flush()
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	accept := acceptKey(key)

	// Write the 101 handshake response directly to the raw connection
	// to avoid any buffering issues
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n" +
		"\r\n"

	if _, err := conn.Write([]byte(handshake)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake write failed: %w", err)
	}

	return &wsConn{
		conn:   conn,
		reader: bufrw.Reader,
		writer: bufio.NewWriter(conn), // fresh writer on raw conn
	}, nil
}

// writeTextFrame writes a WebSocket text frame. Thread-safe.
func (ws *wsConn) writeTextFrame(data []byte) error {
	ws.wmu.Lock()
	defer ws.wmu.Unlock()

	// opcode 0x1 = text frame, FIN bit set
	ws.writer.WriteByte(0x81)

	length := len(data)
	switch {
	case length <= 125:
		ws.writer.WriteByte(byte(length))
	case length <= 65535:
		ws.writer.WriteByte(126)
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(length))
		ws.writer.Write(b[:])
	default:
		ws.writer.WriteByte(127)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(length))
		ws.writer.Write(b[:])
	}

	ws.writer.Write(data)
	return ws.writer.Flush()
}

// readFrame reads a WebSocket frame. Returns opcode and payload.
// Handles masked client frames per RFC 6455.
func (ws *wsConn) readFrame() (opcode byte, payload []byte, err error) {
	// read first 2 bytes
	b0, err := ws.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	b1, err := ws.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	opcode = b0 & 0x0f
	masked := (b1 & 0x80) != 0
	length := uint64(b1 & 0x7f)

	switch length {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(ws.reader, b[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(ws.reader, b[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(b[:])
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(ws.reader, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err := io.ReadFull(ws.reader, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

// writeCloseFrame sends a WebSocket close frame. Thread-safe.
func (ws *wsConn) writeCloseFrame() {
	ws.wmu.Lock()
	defer ws.wmu.Unlock()
	ws.writer.WriteByte(0x88) // FIN + close opcode
	ws.writer.WriteByte(0)    // no payload
	ws.writer.Flush()
}

// writePongFrame sends a WebSocket pong frame. Thread-safe.
func (ws *wsConn) writePongFrame(data []byte) {
	ws.wmu.Lock()
	defer ws.wmu.Unlock()
	ws.writer.WriteByte(0x8A) // FIN + pong opcode
	ws.writer.WriteByte(byte(len(data)))
	ws.writer.Write(data)
	ws.writer.Flush()
}

// Close closes the underlying connection.
func (ws *wsConn) Close() error {
	return ws.conn.Close()
}
