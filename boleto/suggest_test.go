package boleto

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func corrupt(s string, i int) string {
	b := []byte(s)
	b[i] = '0' + (b[i]-'0'+3)%10
	return string(b)
}

func TestSuggestFixBancario(t *testing.T) {
	b, err := NewBancario("341", time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC), 15990, "1090100000012345678900001")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < BancarioLineLen; i++ {
		bad := corrupt(b.DigitableLine, i)
		got := SuggestFix(formatLine(bad))
		if !slices.Contains(got, b.DigitableLine) {
			t.Errorf("position %d: %v does not contain the original", i, got)
		}
		if len(got) > MaxSuggestions {
			t.Errorf("position %d: %d suggestions", i, len(got))
		}
		for _, s := range got {
			if Validate(s) != nil {
				t.Errorf("position %d: invalid suggestion %s", i, s)
			}
		}
	}
}

func TestSuggestFixArrecadacao(t *testing.T) {
	for _, vt := range []int{6, 8} {
		b, err := NewArrecadacao(SegmentPrefeituras, vt, 4321, "1234"+strings.Repeat("5", 25))
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < ArrecadacaoLineLen; i++ {
			got := SuggestFix(corrupt(b.DigitableLine, i))
			// A modulo-11 check digit maps remainders 0 and 1 to the same
			// value, so a few substitutions are undetectable; when the
			// corrupted line is still valid there is nothing to fix.
			if Validate(corrupt(b.DigitableLine, i)) == nil {
				continue
			}
			if !slices.Contains(got, b.DigitableLine) {
				t.Errorf("vt %d position %d: %v does not contain the original", vt, i, got)
			}
		}
	}
}

func TestSuggestFixNoSuggestion(t *testing.T) {
	two := corrupt(corrupt(handLine, 2), 15) // fields 1 and 2 both wrong
	for _, in := range []string{handLine, handBarcode, "abc", "123", two} {
		if got := SuggestFix(in); got != nil {
			t.Errorf("SuggestFix(%q) = %v, want nil", in, got)
		}
	}
}

func TestSuggestFixCap(t *testing.T) {
	// A wrong DAC opens the search to fields 4 and 5, the worst case for
	// the number of candidates; the original must still be included.
	bad := handLine[:32] + "0" + handLine[33:]
	got := SuggestFix(bad)
	if len(got) == 0 || len(got) > MaxSuggestions || !slices.Contains(got, handLine) {
		t.Fatalf("got %d suggestions: %v", len(got), got)
	}
}
