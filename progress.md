1139 baris Go

# DeepL EN→ID (Go)

```
go run . "hello world"            → Halo dunia
go run . <file.srt>               → tulis SRT terjemahan ke stdout
./translate.sh <file.srt>         → tulis <file>.id.srt
```

`./translate.sh Inside.Out...srt` → `Inside.Out...id.srt` (1324 cue, 2295 baris, 35 chunk, ~3 menit).

## Struktur

- `main.go` — CLI: teks biasa atau file `.srt`
- `translate.sh` — wrapper: `./translate.sh <file.srt> [output.srt]`
- `internal/deepl/` — startSession, WebSocket mentah (raw TLS), alur translate
- `internal/msgpack/` — msgpack encode/decode + framing `varint(panjang) + payload`
- `internal/protobuf/` — `BuildAppendRequest`, `ExtractTranslation`
- `internal/srt/` — parse/render `.srt`, potong teks jadi chunk
- Stdlib saja, tanpa dependency eksternal.

## Cara kerja

1. `StartSession()` POST protobuf ke `ita-free.www.deepl.com/v2/startSession` →
   URL `wss://.../v2/sessions?s=...&p=2&i=...&client=...` (token `s`/`i` selalu baru).
2. Buka WebSocket, kirim hello `{"protocol":"messagepack","version":1}`, lalu `[6]`
   dan `Participate` setelah sapaan server.
3. Kirim `AppendRequest` (protobuf di ext type 4); ambil terjemahan dari
   `AppendResponse` (ext type 5) lewat `ExtractTranslation`.

## Catatan penting

- Frame WS itu `varint(panjang payload) + msgpack`. Byte panjang bukan "sequence number";
  hardcoding angka itu (versi lama) membuat teks dengan panjang beda gagal di server.
- **1 sesi DeepL = 1 dokumen.** Sesi tidak bisa dipakai ulang untuk teks lain: server
  membalas gabungan dokumen lama + baru. Jadi sesi diambil fresh per chunk.
  (Percobaan: stream id unik, participant id unik, doc-id AppendRequest, append kosong,
  re-participate — tidak ada yang mengosongkan dokumen.)
- **Batas server 1500 karakter per permintaan** (bukan kata). Lebih dari itu, koneksi
  ditutup (`EOF`). Chunk default 1400 karakter, ubah via `DEEPL_MAX_CHARS`.
- Baris (`\n`) dipertahankan 1:1 oleh server, jadi terjemahan per baris bisa dipetakan
  kembali ke cue. Kalau jumlah baris hasil tak sama, chunk dipecah dua lalu diulang.
- Timeout baca 3 menit; target bahasa hardcoded ID (`0x16 = 22`).
