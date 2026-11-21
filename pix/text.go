package pix

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// asciiFold maps the accented letters used in Portuguese (plus a few common
// neighbours) to their unaccented ASCII form.
var asciiFold = map[rune]byte{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
	'Á': 'A', 'À': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A',
	'É': 'E', 'È': 'E', 'Ê': 'E', 'Ë': 'E',
	'Í': 'I', 'Ì': 'I', 'Î': 'I', 'Ï': 'I',
	'Ó': 'O', 'Ò': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O',
	'Ú': 'U', 'Ù': 'U', 'Û': 'U', 'Ü': 'U',
	'Ç': 'C', 'Ñ': 'N',
}

// toASCII trims s and transliterates accented letters to ASCII. Any other
// character outside printable ASCII (0x20–0x7E) is an error.
func toASCII(s string) (string, error) {
	s = strings.TrimSpace(s)
	if isPrintableASCII(s) {
		return s, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7E:
			b.WriteByte(byte(r))
		case asciiFold[r] != 0:
			b.WriteByte(asciiFold[r])
		case r == utf8.RuneError:
			return "", fmt.Errorf("invalid UTF-8 at byte %d", i)
		default:
			return "", fmt.Errorf("unsupported character %q at byte %d", r, i)
		}
	}
	return b.String(), nil
}

// isPrintableASCII reports whether s contains only bytes 0x20–0x7E.
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
