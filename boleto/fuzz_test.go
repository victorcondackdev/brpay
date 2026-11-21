package boleto

import (
	"strings"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add(handBarcode)
	f.Add(handLine)
	f.Add(handFormatted)
	f.Add(handArrBarcode)
	f.Add(handArrLine)
	f.Add(formatLine(handArrLine))
	f.Add("")
	f.Add("8" + strings.Repeat("9", 47))
	f.Add("00190.00009 00000.500000 00000.000075 9 10000000010000")
	f.Fuzz(func(t *testing.T, s string) {
		b, err := Parse(s)
		_ = SuggestFix(s)
		_, _ = ParseHybrid(s, s)
		if err != nil {
			return
		}
		// A valid code must round-trip through every representation.
		for _, in := range []string{b.Barcode, b.DigitableLine, b.Formatted()} {
			p, err := Parse(in)
			if err != nil {
				t.Fatalf("Parse(%q) of valid %q: %v", in, s, err)
			}
			if p.Barcode != b.Barcode || p.DigitableLine != b.DigitableLine {
				t.Fatalf("round trip mismatch for %q", s)
			}
		}
		if line, err := BarcodeToLine(b.Barcode); err != nil || line != b.DigitableLine {
			t.Fatalf("BarcodeToLine(%q) = %q, %v", b.Barcode, line, err)
		}
		if code, err := LineToBarcode(b.DigitableLine); err != nil || code != b.Barcode {
			t.Fatalf("LineToBarcode(%q) = %q, %v", b.DigitableLine, code, err)
		}
		if b.Bancario != nil {
			_, _ = b.Bancario.DueDate()
		}
		if SuggestFix(b.DigitableLine) != nil {
			t.Fatalf("SuggestFix of a valid line must be nil")
		}
	})
}
