// Package srt: baca/tulis file .srt dan potong teks jadi chunk berbasis jumlah kata.
package srt

import (
	"strings"
)

// Cue = satu blok subtitle.
type Cue struct {
	Index string
	Time  string
	Lines []string
}

// Parse membaca isi file .srt.
func Parse(data []byte) []Cue {
	s := strings.TrimPrefix(string(data), "\ufeff")
	var cues []Cue
	for _, block := range splitBlocks(s) {
		lines := block
		if len(lines) < 2 {
			continue
		}
		cues = append(cues, Cue{Index: lines[0], Time: lines[1], Lines: lines[2:]})
	}
	return cues
}

func splitBlocks(s string) [][]string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var out [][]string
	for _, raw := range strings.Split(s, "\n\n") {
		var lines []string
		for _, l := range strings.Split(raw, "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) > 0 {
			out = append(out, lines)
		}
	}
	return out
}

// Render menulis kembali cue ke format .srt.
func Render(cues []Cue) string {
	var b strings.Builder
	for _, c := range cues {
		b.WriteString(c.Index)
		b.WriteByte('\n')
		b.WriteString(c.Time)
		b.WriteByte('\n')
		b.WriteString(strings.Join(c.Lines, "\n"))
		b.WriteString("\n\n")
	}
	return b.String()
}

// Lines mengumpulkan semua baris teks beserta lokasinya (cue, indeks baris).
func Lines(cues []Cue) (texts []string, at [][2]int) {
	for i, c := range cues {
		for j, l := range c.Lines {
			texts = append(texts, l)
			at = append(at, [2]int{i, j})
		}
	}
	return texts, at
}

// Chunks memotong baris jadi grup; total panjang (termasuk newline) tak melebihi
// maxChars. Server DeepL menolak permintaan di atas 1500 karakter.
func Chunks(lines []string, maxChars int) [][2]int {
	var out [][2]int
	start, size := 0, 0
	for i, l := range lines {
		w := len([]rune(l))
		if i > start && size+1+w > maxChars {
			out = append(out, [2]int{start, i})
			start, size = i, 0
		}
		size += w
		if i > start {
			size++ // newline pemisah
		}
	}
	if start < len(lines) {
		out = append(out, [2]int{start, len(lines)})
	}
	return out
}

// Put menaruh hasil terjemahan kembali ke cue.
func Put(cues []Cue, at [][2]int, texts []string) {
	for k, t := range texts {
		cues[at[k][0]].Lines[at[k][1]] = t
	}
}
