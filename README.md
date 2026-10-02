# DeepL SRT Translator (Go)

Terjemahkan teks dan file subtitle `.srt` ke Bahasa Indonesia lewat endpoint web DeepL — tanpa API key, tanpa dependency eksternal.

## Contoh nama repository

```
deepl-srt-translator
```

Alternatif: `go-deepl-translate`, `deepl-web-translator`, `srt-deepl-id`.

## Deskripsi

> CLI Go (stdlib saja) untuk menerjemahkan teks dan file `.srt` ke Bahasa Indonesia lewat WebSocket web DeepL. Sesi dibuat otomatis tiap permintaan, teks dipotong per chunk agar lolos batas 1500 karakter, dan struktur baris SRT dipertahankan.

Versi pendek untuk GitHub *About*:

> Translate text and .srt subtitle files to Indonesian via DeepL's web endpoint. No API key, no dependencies.

## Tag

```
golang
go
deepl
translator
translation
srt
subtitle
websocket
protobuf
msgpack
cli
no-api-key
bahasa-indonesia
```

## Pakai

```bash
go run . "hello world"                 # Halo dunia
go run . subtitle.srt > subtitle.id.srt
./translate.sh subtitle.srt            # tulis subtitle.id.srt
./translate.sh subtitle.srt out.srt    # tentukan nama output
```

Batasi chunk sendiri (default 1400 karakter, server menolak di atas 1500):

```bash
DEEPL_MAX_CHARS=1200 ./translate.sh subtitle.srt
```

## Fitur

- Teks biasa dan file `.srt` (index + timestamp utuh, teks per baris dipetakan balik ke cue).
- Potong otomatis ≤1400 karakter per permintaan; jumlah baris hasil tidak cocok → chunk dipecah dua dan diulang.
- Sesi (`s`/`i` token) selalu baru per permintaan, jadi tidak ada token statis yang perlu diurus.
- WebSocket mentah di atas TLS supaya header `Origin` bisa diatur, seperti permintaan browser.

## Struktur

```
main.go              CLI: teks atau file .srt
translate.sh         wrapper: ./translate.sh <file.srt> [output.srt]
internal/deepl/      startSession, WebSocket mentah, alur translate
internal/msgpack/    msgpack encode/decode + framing varint(panjang) + payload
internal/protobuf/   BuildAppendRequest, ExtractTranslation
internal/srt/        parse/render .srt, pemotongan chunk
```

## Catatan

- Server membalas **seluruh dokumen** tiap permintaan, dan **1 sesi = 1 dokumen**. Sesi tidak bisa dipakai ulang untuk teks lain, jadi tiap chunk mengambil sesi baru.
- Batas keras server **1500 karakter** per permintaan; lebih dari itu koneksi ditutup (`EOF`).
- Target bahasa masih hardcoded ke Indonesia (`0x16 = 22`) di `AppendRequest`.
- Endpoint web DeepL bisa berubah sewaktu-waktu; kalau protokolnya bergeser, `internal/deepl` perlu disesuaikan.
