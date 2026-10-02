// Package msgpack: msgpack minimal (encode yang dipakai + decode generik) dan
// framing WS DeepL: varint(panjang payload) + payload msgpack.
package msgpack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Ext = msgpack extension (dipakai DeepL untuk protobuf payload).
type Ext struct {
	Type int8
	Data []byte
}

// Map = map berurutan (Go map tak menjamin urutan).
type Map [][2]any

var trueB = []byte{0xc3}
var falseB = []byte{0xc2}
var nilB = []byte{0xc0}

// Encode mengubah nilai Go menjadi msgpack.
func Encode(v any) []byte {
	return encode(nil, v)
}

func encode(out []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(out, nilB...)
	case bool:
		if x {
			return append(out, trueB...)
		}
		return append(out, falseB...)
	case int:
		return encodeInt(out, int64(x))
	case int64:
		return encodeInt(out, x)
	case uint64:
		return encodeUint(out, x)
	case string:
		b := []byte(x)
		if len(b) <= 31 {
			out = append(out, byte(0xa0|len(b)))
		} else {
			out = append(out, 0xda, byte(len(b)>>8), byte(len(b)))
		}
		return append(out, b...)
	case []byte:
		return append(out, x...)
	case Ext:
		d := x.Data
		if len(d) < 256 {
			out = append(out, 0xc7, byte(len(d)), byte(x.Type))
		} else {
			out = append(out, 0xc8, byte(len(d)>>8), byte(len(d)), byte(x.Type))
		}
		return append(out, d...)
	case []any:
		n := len(x)
		if n <= 15 {
			out = append(out, byte(0x90|n))
		} else {
			out = append(out, 0xdc, byte(n>>8), byte(n))
		}
		for _, e := range x {
			out = encode(out, e)
		}
		return out
	case Map:
		n := len(x)
		if n <= 15 {
			out = append(out, byte(0x80|n))
		} else {
			out = append(out, 0xde, byte(n>>8), byte(n))
		}
		for _, kv := range x {
			out = encode(out, kv[0])
			out = encode(out, kv[1])
		}
		return out
	default:
		panic(fmt.Sprintf("msgpack: tipe tak didukung %T", v))
	}
}

func encodeInt(out []byte, x int64) []byte {
	switch {
	case x >= 0:
		return encodeUint(out, uint64(x))
	case x >= -32:
		return append(out, byte(0xe0|(x+32)))
	case x >= math.MinInt8:
		return append(out, 0xd0, byte(int8(x)))
	case x >= math.MinInt16:
		return append(out, 0xd1, byte(x>>8), byte(x))
	case x >= math.MinInt32:
		return append(out, 0xd2, byte(x>>24), byte(x>>16), byte(x>>8), byte(x))
	default:
		return append(out, 0xd3, byte(x>>56), byte(x>>48), byte(x>>40), byte(x>>32), byte(x>>24), byte(x>>16), byte(x>>8), byte(x))
	}
}

func encodeUint(out []byte, x uint64) []byte {
	switch {
	case x <= 0x7f:
		return append(out, byte(x))
	case x <= 0xff:
		return append(out, 0xcc, byte(x))
	case x <= 0xffff:
		return append(out, 0xcd, byte(x>>8), byte(x))
	case x <= 0xffffffff:
		return append(out, 0xce, byte(x>>24), byte(x>>16), byte(x>>8), byte(x))
	default:
		return append(out, 0xcf, byte(x>>56), byte(x>>48), byte(x>>40), byte(x>>32), byte(x>>24), byte(x>>16), byte(x>>8), byte(x))
	}
}

// Decode membaca deretan nilai msgpack sampai buffer habis (0x1e diabaikan).
func Decode(buf []byte) ([]any, error) {
	d := &decoder{buf: buf}
	var out []any
	for d.p < len(buf) {
		if buf[d.p] == 0x1e {
			d.p++
			continue
		}
		v, err := d.val()
		if err != nil {
			return out, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Frame membuang varint panjang di depan, lalu decode payload-nya.
func Frame(buf []byte) ([]any, error) {
	p := 0
	var v uint64
	var s uint
	for p < len(buf) {
		c := buf[p]
		p++
		if s < 64 {
			v |= uint64(c&0x7f) << s
		}
		s += 7
		if c&0x80 == 0 {
			break
		}
	}
	end := p + int(v)
	if end > len(buf) {
		end = len(buf)
	}
	return Decode(buf[p:end])
}

type decoder struct {
	buf []byte
	p   int
}

func (d *decoder) take(n int) ([]byte, error) {
	if d.p+n > len(d.buf) {
		return nil, errors.New("msgpack: buffer kurang")
	}
	b := d.buf[d.p : d.p+n]
	d.p += n
	return b, nil
}

func (d *decoder) byte1() (byte, error) {
	if d.p >= len(d.buf) {
		return 0, errors.New("msgpack: opcode hilang")
	}
	b := d.buf[d.p]
	d.p++
	return b, nil
}

func (d *decoder) val() (any, error) {
	b, err := d.byte1()
	if err != nil {
		return nil, err
	}
	switch {
	case b <= 0x7f:
		return int64(b), nil
	case b >= 0xe0:
		return int64(int8(b)), nil
	case b >= 0xa0 && b <= 0xbf:
		x, err := d.take(int(b & 0x1f))
		return string(x), err
	case b >= 0x90 && b <= 0x9f:
		return d.array(int(b & 0x0f))
	case b >= 0x80 && b <= 0x8f:
		return d.mp(int(b & 0x0f))
	}
	switch b {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4, 0xc5, 0xc6:
		return d.bin(b)
	case 0xc7, 0xc8, 0xc9:
		return d.ext(b)
	case 0xca:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(x))), nil
	case 0xcb:
		x, err := d.take(8)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(x)), nil
	case 0xcc, 0xcd, 0xce, 0xcf:
		return d.uint(b)
	case 0xd0, 0xd1, 0xd2, 0xd3:
		return d.int(b)
	case 0xd9, 0xda, 0xdb:
		return d.str(b)
	case 0xdc, 0xdd:
		return d.array32(b)
	case 0xde, 0xdf:
		return d.mp32(b)
	}
	if b >= 0xd4 && b <= 0xd8 {
		n := []int{1, 2, 4, 8, 16}[b-0xd4]
		t, err := d.byte1()
		if err != nil {
			return nil, err
		}
		data, err := d.take(n)
		if err != nil {
			return nil, err
		}
		return Ext{Type: int8(t), Data: data}, nil
	}
	return nil, fmt.Errorf("msgpack: opcode 0x%x", b)
}

func (d *decoder) array(n int) (any, error) { return d.arrayN(n) }

func (d *decoder) arrayN(n int) (any, error) {
	a := make([]any, 0, n)
	for i := 0; i < n; i++ {
		v, err := d.val()
		if err != nil {
			return nil, err
		}
		a = append(a, v)
	}
	return a, nil
}

func (d *decoder) mp(n int) (any, error) {
	m := make(Map, 0, n)
	for i := 0; i < n; i++ {
		k, err := d.val()
		if err != nil {
			return nil, err
		}
		v, err := d.val()
		if err != nil {
			return nil, err
		}
		m = append(m, [2]any{k, v})
	}
	return m, nil
}

func (d *decoder) bin(b byte) (any, error) {
	var n int
	switch b {
	case 0xc4:
		x, err := d.byte1()
		if err != nil {
			return nil, err
		}
		n = int(x)
	case 0xc5:
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(x))
	default:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint32(x))
	}
	return d.take(n)
}

func (d *decoder) ext(b byte) (any, error) {
	var n int
	switch b {
	case 0xc7:
		x, err := d.byte1()
		if err != nil {
			return nil, err
		}
		n = int(x)
	case 0xc8:
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(x))
	default:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint32(x))
	}
	t, err := d.byte1()
	if err != nil {
		return nil, err
	}
	data, err := d.take(n)
	if err != nil {
		return nil, err
	}
	return Ext{Type: int8(t), Data: data}, nil
}

func (d *decoder) uint(b byte) (any, error) {
	switch b {
	case 0xcc:
		x, err := d.byte1()
		return int64(x), err
	case 0xcd:
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint16(x)), nil
	case 0xce:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint32(x)), nil
	default:
		x, err := d.take(8)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint64(x), nil
	}
}

func (d *decoder) int(b byte) (any, error) {
	switch b {
	case 0xd0:
		x, err := d.byte1()
		return int64(int8(x)), err
	case 0xd1:
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		return int64(int16(binary.BigEndian.Uint16(x))), nil
	case 0xd2:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		return int64(int32(binary.BigEndian.Uint32(x))), nil
	default:
		x, err := d.take(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(x)), nil
	}
}

func (d *decoder) str(b byte) (any, error) {
	var n int
	switch b {
	case 0xd9:
		x, err := d.byte1()
		if err != nil {
			return nil, err
		}
		n = int(x)
	case 0xda:
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(x))
	default:
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint32(x))
	}
	x, err := d.take(n)
	return string(x), err
}

func (d *decoder) array32(b byte) (any, error) {
	var n int
	if b == 0xdc {
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(x))
	} else {
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint32(x))
	}
	return d.arrayN(n)
}

func (d *decoder) mp32(b byte) (any, error) {
	var n int
	if b == 0xde {
		x, err := d.take(2)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(x))
	} else {
		x, err := d.take(4)
		if err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint32(x))
	}
	return d.mp(n)
}
