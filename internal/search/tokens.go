package search

import (
	"strings"
)

// tokenize lowers ASCII and splits on separators.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	repl := func(r rune) rune {
		switch r {
		case '/', '.', '_', '-', ' ':
			return ' '
		default:
			if r < 128 {
				return r
			}
			return ' '
		}
	}
	s = strings.Map(repl, s)
	return strings.Fields(s)
}

// buildQueryNgrams returns query ngrams for fuzzy search.
// ASCII: lowercase; split by separators; 2-gram for tokens with length >= 3
// CJK: bigram on contiguous CJK sequences without using any dictionary
func buildQueryNgrams(s string) []string {
	s = strings.ToLower(s)
	toks := tokenize(s)
	out := make([]string, 0, 16)
	// ASCII 2-gram
	for _, t := range toks {
		if len(t) < 3 {
			continue
		}
		for i := 0; i+2 <= len(t); i++ {
			out = append(out, t[i:i+2])
		}
	}
	// CJK bigrams from original string
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if isCJK(runes[i]) {
			j := i + 1
			for j < len(runes) && isCJK(runes[j]) {
				j++
			}
			if j-i >= 2 {
				for k := i; k+1 < j; k++ {
					out = append(out, string(runes[k:k+2]))
				}
			}
			i = j
		} else {
			i++
		}
	}
	return out
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0x3040 && r <= 0x309F:
		return true
	case r >= 0x30A0 && r <= 0x30FF:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	default:
		return false
	}
}
