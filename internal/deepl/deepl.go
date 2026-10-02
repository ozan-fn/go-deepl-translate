// Package deepl: DeepL web EN->ID lewat startSession (protobuf) + WebSocket (msgpack).
package deepl

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"deepl/internal/msgpack"
	"deepl/internal/protobuf"
)

const (
	ua       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36 Edg/154.0.0.0"
	clientID = "1e1e692b-ff5f-4b74-86f2-b66d2544253e"
	startURL = "https://ita-free.www.deepl.com/v2/startSession?client=" + clientID
	stableID = "8bd6d20b-b139-4bbd-b850-a7917dfb7569" // x-statsig-stable-id
	wsHost   = "ita-free.www.deepl.com"
)

// Body protobuf startSession, base64.
const startBodyB64 = "CAESlhEKkQoIASK4BQgBErMFCgQKAnN0CgQKAnRsCgQKAmZyCgUKA2NrYgoECgJzcQoECgJjYQoECgJwcwoECgJteQoECgJzYQoFCgNtYWkKBAoCaHIKBAoCaXMKBQoDcHJzCgQKAmFyCgUKA2NlYgoECgJlbAoFCgNwYWcKBAoCaHkKBAoCbmUKBAoCdHMKBAoCaGUKBAoCbmwKBAoCZGUKBAoCbHQKBQoDa21yCgQKAmhhCgQKAnZpCgQKAm1pCgQKAnJvCgQKAm1rCgQKAmJhCgQKAmN5CgQKAnp1CgQKAnB0CgQKAmtrCgQKAmV0CgQKAm9jCgQKAmVzCgQKAnRrCgQKAmFuCgQKAmF6CgUKA3l1ZQoECgJydQoFCgNnb20KBAoCcGwKBAoCc3cKBAoCc2wKBAoCc2sKBAoCdGUKBAoCYXMKBAoCbWwKBAoCYnMKBAoCdXIKBAoCaXQKBAoCbmIKBAoCdG4KBAoCdHIKBQoDc2NuCgQKAmZhCgQKAndvCgUKA3BhbQoECgJpZAoECgJlbwoECgJhZgoECgJibgoECgJndQoECgJnbgoECgJtcgoECgJrbwoECgJqYQoECgJmaQoFCgNhY2UKBAoCZXUKBAoCc3IKBAoCb20KBAoCdGcKBAoCYmcKBAoCYnIKBAoCeWkKBQoDbG1vCgQKAmh1CgQKAnhoCgQKAmRhCgQKAnN2CgQKAmt5CgQKAmxhCgQKAmdsCgQKAmxuCgQKAm1nCgQKAmJlCgQKAnpoCgQKAm1zCgQKAmh0CgQKAm1uCgQKAmVuCgUKA2JobwoECgJqdgoECgJjcwoECgJ0dAoECgJ0YQoECgJsdgoECgJzdQoECgJpZwoECgJnYQoECgJ1awoECgJwYQoECgJsYgoECgJoaQoECgJheQoECgJxdQoECgJtdAoECgJ1egoECgJrYSIHCA5yAwjcCyIKCAMiBgoECgJlbiIMCARqCAoECgJlbhABIgYIDGICCAEipgQIHvIBoAQKDwoECgJhchHy7zMuHAipPwoPCgQKAmJnEQYv+grSjKU/Cg8KBAoCY3MR0bNZ9bnasj8KDwoECgJkYREOpmH4iJiyPwoPCgQKAmRlEbK/7J48LMQ/Cg8KBAoCZWwRf4eiQJ/Ioz8KDwoECgJlbhFBguLHmLvzPwoPCgQKAmVzEaezk8FR8sI/Cg8KBAoCZXQRWMoyxLEurj8KDwoECgJmaRHmP6Tfvg60PwoPCgQKAmZyESkn2lVI+cE/Cg8KBAoCaHURuVM6WP/nsD8KDwoECgJpZBHb4a/JGvWwPwoPCgQKAml0EWQGKuPfZ7w/Cg8KBAoCamERiWNd3EYDwD8KDwoECgJrbxGOI9biUwCsPwoPCgQKAmx0EcJWCRaHM68/Cg8KBAoCbHYRLbe0GhL3qD8KDwoECgJuYhHx12SNeoi2PwoPCgQKAm5sEWQoJ9pVSLk/Cg8KBAoCcGwRHjhnRGlvuD8KDwoECgJwdBGhZ7Pqc7W1PwoPCgQKAnJvEfM8uDtrt60/Cg8KBAoCcnURF/a0w1+TtT8KDwoECgJzaxGNKO0NvjCxPwoPCgQKAnNsEbubpzrkZrA/Cg8KBAoCc3YRG91B7Eyhsz8KDwoECgJ0chGNRdPZyeCwPwoPCgQKAnVrEeNw5ldzgLA/Cg8KBAoCemgRBqOSOgFNwD8KDwoECgJ2aRE6I0p7gy+sPwoPCgQKAmhlEbx+wW7Ytqg/Cv8GCAIi/AUIAhr3BQoECgJzdAoECgJrYQoECgJmaQoECgJldQoECgJzYQoHCgVkZS1DSAoECgJteQoECgJoYQoECgJjcwoECgJtZwoECgJhbgoFCgNrbXIKBAoCc3UKBAoCYXMKBAoCbHYKBAoCbmUKBAoCdHIKBAoCcHMKBAoCcXUKBAoCdGcKBAoCc3cKBAoCamEKBAoCYm4KBAoCenUKBAoCc2sKBAoCdXoKBAoCbmIKBAoCaHUKBAoCdGEKBAoCa3kKBAoCcm8KBAoCaWcKBAoCZmEKBAoCZ3UKBAoCbWwKBQoDbWFpCgUKA2NlYgoFCgNzY24KBAoCc3EKBAoCdG4KBQoDbG1vCgUKA2NrYgoECgJ5aQoECgJ1cgoECgJ0YQoJCgd6aC1IYW50CgQKAnVrCgQKAmxuCgQKAmh0CgQKAmVsCgUKA3BhbQoECgJrawoECgJubAoECgJpZAoECgJocgoECgJrbwoECgJ0bAoECgJnbAoECgJnbgoECgJkZQoECgJtcgoECgJydQoECgJjeQoECgJlbwoECgJzcgoECgJjYQoECgJsdAoECgJvYwoHCgVlbi1HQgoECgJvbQoECgJiYQoECgJmcgoECgJpcwoHCgVwdC1CUgoECgJoeQoECgJicwoHCgVlbi1VUwoECgJsYgoFCgNnb20KBAoCcGEKBAoCbWkKBAoCYmcKCAoGZXMtNDE5CgQKAmV0CgQKAnhoCgQKAm1rCgUKA2FjZQoECgJ2aQoECgJiZQoHCgVwdC1QVAoECgJlcwoECgJzbAoFCgNiaG8KBAoCdGUKBAoCc3YKBAoCbXQKCQoHemgtSGFucwoFCgN5dWUKBAoCZ2EKBAoCbW4KBAoCYXIKBAoCaXQKBAoCdGsKBAoCcGwKBAoCYWYKBAoCdHQKBAoCanYKBAoCYXoKBAoCdHMKBQoDcGFnCgQKAmJyCgQKAmhpCgQKAmF5CgQKAndvCgQKAmhlCgQKAm1zCgUKA3BycwoECgJsYQoHCgVmci1DQSIFCCOaAgAiBQgkogIAIgoIBSoGCgQKAmlkIgQICUoAIgYIDGICCAEiBQgs4gIAIgcIDnIDCPAuIhAID3oMCgoKCG5leHQtZ2VuIgsIEpIBBgoECgJpZCIJCDKSAwQIBBIAIgcIM5oDAgoAIhEIEIIBDAoKCghuZXh0LWdlbhoCEAE="

var (
	bodyOnce sync.Once
	bodyBuf  []byte
)

func startBody() []byte {
	bodyOnce.Do(func() {
		b, err := base64.StdEncoding.DecodeString(startBodyB64)
		if err != nil {
			panic(err)
		}
		bodyBuf = b
	})
	return bodyBuf
}

// StartSession memulai sesi baru dan mengembalikan URL WebSocket (token selalu fresh).
func StartSession() (string, error) {
	req, err := http.NewRequest("POST", startURL, bytes.NewReader(startBody()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Origin", "https://www.deepl.com")
	req.Header.Set("Referer", "https://www.deepl.com/")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("x-statsig-stable-id", stableID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("startSession HTTP %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	for _, f := range protobuf.Walk(buf) {
		if f.F != 5 {
			continue
		}
		if p, ok := f.V.([]byte); ok {
			s := string(p)
			if strings.HasPrefix(s, "/v2/sessions") {
				return "wss://" + wsHost + s + "&client=" + clientID, nil
			}
		}
	}
	return "", errors.New("session path tak ketemu di respons startSession")
}

// Translate menerjemahkan text EN->ID. Satu sesi hanya untuk satu teks.
func Translate(text string) (string, error) {
	sessURL, err := StartSession()
	if err != nil {
		return "", err
	}
	conn, err := dial(sessURL)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.WriteText(`{"protocol":"messagepack","version":1}` + "\x1e"); err != nil {
		return "", err
	}

	base := time.Now().UnixNano() / int64(time.Millisecond) % 1e9
	pid := fmt.Sprint(base)
	stream := fmt.Sprint(base + 1)

	var mu sync.Mutex
	appendSent := false
	sendAppend := func() {
		data := protobuf.BuildAppendRequest(text, 1)
		frame := msgpack.Encode([]any{int64(1), msgpack.Map{}, stream, "AppendRequest", []any{msgpack.Ext{Type: 4, Data: data}}})
		_ = conn.WriteBinary(prefixed(frame))
	}
	markAppend := func() {
		mu.Lock()
		defer mu.Unlock()
		if !appendSent {
			appendSent = true
			sendAppend()
		}
	}
	go func() { time.Sleep(2 * time.Second); markAppend() }() // fallback jika ack participate tak datang

	var got string
	var lastGot time.Time
	overall := time.Now().Add(3 * time.Minute) // chunk besar (ratusan baris) butuh waktu

	for {
		dl := overall
		if !lastGot.IsZero() {
			if t2 := lastGot.Add(800 * time.Millisecond); t2.Before(dl) {
				dl = t2
			}
		}
		_ = conn.SetReadDeadline(dl)

		op, payload, err := conn.ReadMessage()
		if err != nil {
			var ne net.Error
			if (errors.As(err, &ne) && ne.Timeout()) || errors.Is(err, io.EOF) {
				if got != "" && time.Now().After(lastGot) {
					return got, nil
				}
			}
			if got != "" {
				return got, nil
			}
			return "", err
		}
		if op != 0x2 || len(payload) == 0 {
			continue
		}
		if payload[0] == 0x7b { // `{}` sapaan server -> ack + participate
			_ = conn.WriteBinary(prefixed(msgpack.Encode([]any{int64(6)})))
			_ = conn.WriteBinary(prefixed(msgpack.Encode([]any{int64(1), msgpack.Map{}, pid, "Participate", []any{msgpack.Ext{Type: 3, Data: nil}}})))
			continue
		}
		msgs, err := msgpack.Frame(payload)
		if err != nil {
			continue
		}
		for _, raw := range msgs {
			m, ok := raw.([]any)
			if !ok || len(m) < 4 {
				continue
			}
			if code, ok := m[3].(int64); ok && code == 3 {
				if s, ok := m[2].(string); ok && s == pid {
					markAppend()
				}
				continue
			}
			if s, ok := m[3].(string); ok && s == "AppendResponse" {
				arr, ok := m[4].([]any)
				if !ok {
					continue
				}
				for _, a := range arr {
					if ext, ok := a.(msgpack.Ext); ok && ext.Type == 5 {
						if t := protobuf.ExtractTranslation(ext.Data); t != "" {
							got = t
							lastGot = time.Now()
						}
					}
				}
			}
		}
	}
}

func prefixed(payload []byte) []byte {
	return append(protobuf.Varint(uint64(len(payload))), payload...)
}

// --- WebSocket mentah via tls (untuk set header Origin) ---

type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
	mu   sync.Mutex
}

func dial(wsURL string) (*wsConn, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, err
	}
	addr := u.Host
	if u.Port() == "" {
		addr = net.JoinHostPort(u.Hostname(), "443")
	}
	raw, err := tls.Dial("tcp", addr, &tls.Config{ServerName: u.Hostname()})
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	if os.Getenv("WSDEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[ws] GET %s?%s\n", u.Path, u.RawQuery)
	}
	req := fmt.Sprintf("GET %s?%s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nOrigin: https://www.deepl.com\r\nUser-Agent: %s\r\n\r\n",
		u.Path, u.RawQuery, u.Host, base64.StdEncoding.EncodeToString(key), ua)
	if _, err := raw.Write([]byte(req)); err != nil {
		raw.Close()
		return nil, err
	}
	br := bufio.NewReader(raw)
	status, err := br.ReadString('\n')
	if err != nil {
		raw.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		raw.Close()
		return nil, errors.New("WS handshake ditolak: " + strings.TrimSpace(status))
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			raw.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return &wsConn{conn: raw, br: br}, nil
}

func (c *wsConn) Close() { c.conn.Close() }

func (c *wsConn) SetReadDeadline(t time.Time) error { return c.conn.SetReadDeadline(t) }

func (c *wsConn) WriteText(s string) error { return c.writeFrame(0x1, []byte(s)) }

func (c *wsConn) WriteBinary(b []byte) error { return c.writeFrame(0x2, b) }

func (c *wsConn) writeFrame(op byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	head := []byte{0x80 | op}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	n := len(payload)
	switch {
	case n < 126:
		head = append(head, 0x80|byte(n))
	case n < 65536:
		head = append(head, 0x80|126, byte(n>>8), byte(n))
	default:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		head = append(head, 0x80|127)
		head = append(head, b[:]...)
	}
	masked := make([]byte, n)
	for i := 0; i < n; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := c.conn.Write(head); err != nil {
		return err
	}
	if _, err := c.conn.Write(mask); err != nil {
		return err
	}
	_, err := c.conn.Write(masked)
	return err
}

// ReadMessage mengembalikan satu pesan lengkap (menggabungkan fragment, menjawab ping).
func (c *wsConn) ReadMessage() (byte, []byte, error) {
	var op byte
	var out []byte
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(c.br, h); err != nil {
			return 0, nil, err
		}
		fin := h[0]&0x80 != 0
		cur := h[0] & 0x0f
		masked := h[1]&0x80 != 0
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(c.br, b[:]); err != nil {
				return 0, nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(c.br, b[:]); err != nil {
				return 0, nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		var key []byte
		if masked {
			key = make([]byte, 4)
			if _, err := io.ReadFull(c.br, key); err != nil {
				return 0, nil, err
			}
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(c.br, data); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range data {
				data[i] ^= key[i%4]
			}
		}
		switch cur {
		case 0x8: // close
			return 0, nil, io.EOF
		case 0x9: // ping -> pong
			_ = c.writeFrame(0xa, data)
			continue
		case 0xa: // pong
			continue
		case 0x0: // continuation
			out = append(out, data...)
		default:
			op = cur
			out = append(out, data...)
		}
		if fin {
			return op, out, nil
		}
	}
}
