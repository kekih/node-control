package agent

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

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// WSConn represents a WebSocket connection over hijacked TCP connection
type WSConn struct {
	conn   net.Conn
	rw     *bufio.ReadWriter
	closed bool
	mu     sync.Mutex
}

// UpgradeWebSocket upgrades an HTTP connection to RFC 6455 WebSocket
func UpgradeWebSocket(w http.ResponseWriter, r *http.Request) (*WSConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("missing or invalid Upgrade header")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key header")
	}

	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("responseWriter does not support hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	// Send handshake response
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := rw.WriteString(handshake); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	ws := &WSConn{
		conn: conn,
		rw:   rw,
	}
	return ws, nil
}

// WriteText sends a text frame (opcode 0x1) to the client unmasked
func (ws *WSConn) WriteText(msg string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return net.ErrClosed
	}

	data := []byte(msg)
	length := len(data)

	// Byte 0: FIN (0x80) | Text (0x01) = 0x81
	if err := ws.rw.WriteByte(0x81); err != nil {
		return err
	}

	// Length encoding
	if length < 126 {
		if err := ws.rw.WriteByte(byte(length)); err != nil {
			return err
		}
	} else if length <= 65535 {
		if err := ws.rw.WriteByte(126); err != nil {
			return err
		}
		var lenBuf [2]byte
		binary.BigEndian.PutUint16(lenBuf[:], uint16(length))
		if _, err := ws.rw.Write(lenBuf[:]); err != nil {
			return err
		}
	} else {
		if err := ws.rw.WriteByte(127); err != nil {
			return err
		}
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(length))
		if _, err := ws.rw.Write(lenBuf[:]); err != nil {
			return err
		}
	}

	if _, err := ws.rw.Write(data); err != nil {
		return err
	}
	return ws.rw.Flush()
}

// ReadLoop listens for frames, answers Ping with Pong, and exits on Close or disconnect
func (ws *WSConn) ReadLoop() {
	defer ws.Close()
	for {
		b1, err := ws.rw.ReadByte()
		if err != nil {
			return
		}
		b2, err := ws.rw.ReadByte()
		if err != nil {
			return
		}

		opcode := b1 & 0x0F
		masked := (b2 & 0x80) != 0
		payloadLen := int(b2 & 0x7F)

		if payloadLen == 126 {
			var l [2]byte
			if _, err := io.ReadFull(ws.rw, l[:]); err != nil {
				return
			}
			payloadLen = int(binary.BigEndian.Uint16(l[:]))
		} else if payloadLen == 127 {
			var l [8]byte
			if _, err := io.ReadFull(ws.rw, l[:]); err != nil {
				return
			}
			payloadLen = int(binary.BigEndian.Uint64(l[:]))
		}

		var maskKey [4]byte
		if masked {
			if _, err := io.ReadFull(ws.rw, maskKey[:]); err != nil {
				return
			}
		}

		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(ws.rw, payload); err != nil {
			return
		}

		if masked {
			for i := 0; i < payloadLen; i++ {
				payload[i] ^= maskKey[i%4]
			}
		}

		if opcode == 0x8 { // Close frame
			return
		}
		if opcode == 0x9 { // Ping frame -> respond with Pong (0x8A)
			ws.mu.Lock()
			_ = ws.rw.WriteByte(0x8A)
			_ = ws.rw.WriteByte(0x00)
			_ = ws.rw.Flush()
			ws.mu.Unlock()
		}
	}
}

// Close closes the WebSocket
func (ws *WSConn) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.closed {
		ws.closed = true
		return ws.conn.Close()
	}
	return nil
}
