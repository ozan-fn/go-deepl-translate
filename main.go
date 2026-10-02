package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"deepl/internal/deepl"
	"deepl/internal/srt"
)

// Server DeepL membatasi 1500 karakter per permintaan; sisakan margin.
const defaultMaxChars = 1400

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, `Usage: go run . "<teks>"`)
		fmt.Fprintln(os.Stderr, `   or: go run . <file.srt>`)
		os.Exit(1)
	}
	arg := os.Args[1]
	if fi, err := os.Stat(arg); err == nil && !fi.IsDir() {
		if err := translateSRT(arg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	out, err := deepl.Translate(arg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(out)
}

func translateSRT(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cues := srt.Parse(data)
	if len(cues) == 0 {
		return fmt.Errorf("tak ada cue di %s", path)
	}
	lines, at := srt.Lines(cues)
	maxChars := defaultMaxChars
	if v := os.Getenv("DEEPL_MAX_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxChars = n
		}
	}
	chunks := srt.Chunks(lines, maxChars)
	fmt.Fprintf(os.Stderr, "%s: %d cue, %d baris, %d chunk (max %d karakter)\n", path, len(cues), len(lines), len(chunks), maxChars)

	for i, c := range chunks {
		piece := lines[c[0]:c[1]]
		fmt.Fprintf(os.Stderr, "chunk %d/%d (%d baris)...\n", i+1, len(chunks), len(piece))
		got, err := translateLines(piece)
		if err != nil {
			return fmt.Errorf("chunk %d/%d: %w", i+1, len(chunks), err)
		}
		srt.Put(cues, at[c[0]:c[1]], got)
	}
	fmt.Print(srt.Render(cues))
	return nil
}

// translateLines menerjemahkan baris sekaligus; kalau jumlah baris hasil tak
// sama dengan input, chunk dipecah dua sampai cocok.
func translateLines(lines []string) ([]string, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	out, err := deepl.Translate(strings.Join(lines, "\n"))
	if err != nil {
		return nil, err
	}
	got := strings.Split(out, "\n")
	if len(got) == len(lines) {
		return got, nil
	}
	if len(lines) == 1 {
		return []string{out}, nil
	}
	mid := len(lines) / 2
	a, err := translateLines(lines[:mid])
	if err != nil {
		return nil, err
	}
	b, err := translateLines(lines[mid:])
	if err != nil {
		return nil, err
	}
	return append(a, b...), nil
}
