package dashboard

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
)

// Minimal WebSocket implementation (RFC 6455) — no external dependencies.
// Supports text frames only, which is sufficient for JSON event streaming.

const websocketGUID = "258EAFA5-E914-47DA-95CA-5AB5DC525C63"

func isWebSocketUpgrade(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket" &&
		r.Header.Get("Connection") != "" &&
		r.Header.Get("Sec-WebSocket-Key") != ""
}

func acceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (io.ReadWriteCloser, *bufio.ReadWriter, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket upgrade not supported", http.StatusInternalServerError)
		return nil, nil, fmt.Errorf("hijack not supported")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	accept := acceptKey(key)

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n")
	bufrw.WriteString("\r\n")
	bufrw.Flush()

	return conn, bufrw, nil
}

// writeTextFrame writes a WebSocket text frame.
func writeTextFrame(bufrw *bufio.ReadWriter, data []byte) error {
	// opcode 0x1 = text frame, FIN bit set
	bufrw.WriteByte(0x81)

	length := len(data)
	switch {
	case length <= 125:
		bufrw.WriteByte(byte(length))
	case length <= 65535:
		bufrw.WriteByte(126)
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(length))
		bufrw.Write(b[:])
	default:
		bufrw.WriteByte(127)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(length))
		bufrw.Write(b[:])
	}

	bufrw.Write(data)
	return bufrw.Flush()
}

// readFrame reads a WebSocket frame. Returns opcode and payload.
// Handles masked client frames per RFC 6455.
func readFrame(bufrw *bufio.ReadWriter) (opcode byte, payload []byte, err error) {
	// read first 2 bytes
	b0, err := bufrw.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	b1, err := bufrw.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	opcode = b0 & 0x0f
	masked := (b1 & 0x80) != 0
	length := uint64(b1 & 0x7f)

	switch length {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(bufrw, b[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(bufrw, b[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(b[:])
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(bufrw, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err := io.ReadFull(bufrw, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

// writeCloseFrame sends a WebSocket close frame.
func writeCloseFrame(bufrw *bufio.ReadWriter) {
	bufrw.WriteByte(0x88) // FIN + close opcode
	bufrw.WriteByte(0)    // no payload
	bufrw.Flush()
}

// writePongFrame sends a WebSocket pong frame with the given payload.
func writePongFrame(bufrw *bufio.ReadWriter, data []byte) {
	bufrw.WriteByte(0x8A) // FIN + pong opcode
	bufrw.WriteByte(byte(len(data)))
	bufrw.Write(data)
	bufrw.Flush()
}
