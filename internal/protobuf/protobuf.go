// Package protobuf: protobuf minimal untuk startSession dan pesan AppendRequest.
package protobuf

import (
	"regexp"
	"strings"
)

// Field = hasil Walk: V berisi uint64 (varint) atau []byte (length-delimited).
type Field struct {
	F int
	V any
}

// Varint meng-encode bilangan bulat tanpa tanda.
func Varint(n uint64) []byte {
	var b []byte
	for {
		c := byte(n & 0x7f)
		n >>= 7
		if n != 0 {
			c |= 0x80
		}
		b = append(b, c)
		if n == 0 {
			return b
		}
	}
}

// Fld menulis field length-delimited bernomor n.
func Fld(n int, payload []byte) []byte {
	out := Varint(uint64(n)<<3 | 2)
	out = append(out, Varint(uint64(len(payload)))...)
	return append(out, payload...)
}

// Walk membaca field protobuf secara generik.
func Walk(buf []byte) []Field {
	var out []Field
	p := 0
	for p < len(buf) {
		key, n := readVarint(buf[p:])
		if n == 0 {
			break
		}
		p += n
		f, w := int(key>>3), key&7
		switch w {
		case 0:
			v, n := readVarint(buf[p:])
			p += n
			out = append(out, Field{f, v})
		case 2:
			l, n := readVarint(buf[p:])
			p += n
			end := p + int(l)
			if end > len(buf) {
				return out
			}
			out = append(out, Field{f, buf[p:end]})
			p = end
		case 5:
			p += 4
		case 1:
			p += 8
		default:
			return out
		}
	}
	return out
}

func readVarint(buf []byte) (uint64, int) {
	var v uint64
	var s uint
	for i := 0; i < len(buf); i++ {
		c := buf[i]
		v |= uint64(c&0x7f) << s
		if c&0x80 == 0 {
			return v, i + 1
		}
		s += 7
	}
	return v, 0
}

// BuildAppendRequest menyusun payload AppendRequest (ext4) untuk sepotong teks.
func BuildAppendRequest(text string, docID uint64) []byte {
	inner := []byte{0x08}
	inner = append(inner, Varint(docID)...)
	inner = append(inner, Fld(2, append(Fld(1, nil), Fld(2, []byte(text))...))...)
	inner = append(inner, Fld(6, []byte{0x08, 0x02})...)
	body := append(Fld(1, inner), Fld(2, Fld(1, []byte{0x08, 0x16}))...)
	return Fld(1, body)
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

// ExtractTranslation mengambil teks terjemahan dari payload AppendResponse (ext5).
func ExtractTranslation(ext []byte) string {
	var found []string
	for _, m3 := range Walk(ext) {
		b3, ok := m3.V.([]byte)
		if m3.F != 3 || !ok {
			continue
		}
		for _, m1 := range Walk(b3) {
			b1, ok := m1.V.([]byte)
			if m1.F != 1 || !ok {
				continue
			}
			for _, m2 := range Walk(b1) {
				b2, ok := m2.V.([]byte)
				if m2.F != 2 || !ok {
					continue
				}
				for _, s := range Walk(b2) {
					bs, ok := s.V.([]byte)
					if s.F != 2 || !ok || len(bs) == 0 {
						continue
					}
					str := string(bs)
					if !uuidRe.MatchString(str) {
						found = append(found, str)
					}
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(found, " "))
}
