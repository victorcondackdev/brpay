package boleto

import (
	"errors"
	"strings"
	"testing"
)

// Hand-computed arrecadação vector (modulo 10).
//
// Product 8, segment 2 (saneamento), value type 6 (effective value, mod 10),
// value 00000001000 (R$ 10,00), company 0001, free field of zeros. Without
// the general DV the non-zero digits and their mod-10 weights (from the
// right, 2,1,2,1…) are
//
//	1 in company (pos 25, w 1) = 1
//	1 in value   (pos 32, w 2) = 2
//	6 value type (pos 40, w 2) = 12 → 1+2 = 3
//	2 segment    (pos 41, w 1) = 2
//	8 product    (pos 42, w 2) = 16 → 1+6 = 7
//	sum = 15 → DV = (10 − 5) % 10 = 5
//
// Block 1 "82650000000": 5×1, 6×2→3, 2×1, 8×2→7 = 17 → DV 3.
// Block 2 "10000001000": 1×2 = 2 and 1×1 = 1 → 3 → DV 7.
// Blocks 3 and 4 are zeros → DV 0.
var (
	handArrBarcode = "8265" + "00000001000" + "0001" + strings.Repeat("0", 25)
	handArrLine    = "826500000003" + "100000010007" + strings.Repeat("0", 24)
)

func TestHandComputedArrecadacao(t *testing.T) {
	b, err := Parse(handArrBarcode)
	if err != nil {
		t.Fatal(err)
	}
	if b.DigitableLine != handArrLine {
		t.Errorf("line = %s, want %s", b.DigitableLine, handArrLine)
	}
	want := "82650000000-3 10000001000-7 00000000000-0 00000000000-0"
	if b.Formatted() != want {
		t.Errorf("formatted = %q, want %q", b.Formatted(), want)
	}
	a := b.Arrecadacao
	if b.Type != TypeArrecadacao || b.AmountCents != 1000 || a.Segment != SegmentSaneamento ||
		a.ValueIsReference || a.Modulus != 10 || a.CompanyID != "0001" || a.CNPJBase != "" {
		t.Errorf("unexpected %+v %+v", b, a)
	}
	if a.Segment.String() != "Saneamento" {
		t.Errorf("segment name = %q", a.Segment)
	}
}

func TestArrecadacaoBothModuli(t *testing.T) {
	rest := "0123" + "4567890123456789012345678" // company + free field (29)
	for _, vt := range []int{6, 7, 8, 9} {
		// Build the 43 digits by hand and compute the DV independently of
		// NewArrecadacao, using the modulus the value type selects.
		body := "83" + string(rune('0'+vt)) + "00000123456" + rest
		var dv byte
		if vt <= 7 {
			dv = mod10(body)
		} else {
			dv = mod11Arrecadacao(body)
		}
		code := body[:3] + string(dv) + body[3:]

		line, err := BarcodeToLine(code)
		if err != nil {
			t.Fatalf("vt %d: BarcodeToLine: %v", vt, err)
		}
		if len(line) != ArrecadacaoLineLen {
			t.Fatalf("vt %d: line length %d", vt, len(line))
		}
		back, err := LineToBarcode(formatLine(line))
		if err != nil || back != code {
			t.Fatalf("vt %d: LineToBarcode = %s, %v; want %s", vt, back, err, code)
		}
		b, err := NewArrecadacao(SegmentEnergiaGas, vt, 123456, rest)
		if err != nil || b.Barcode != code {
			t.Fatalf("vt %d: NewArrecadacao = %v, %v; want %s", vt, b, err, code)
		}
		a := b.Arrecadacao
		wantMod := 10
		if vt >= 8 {
			wantMod = 11
		}
		if a.Modulus != wantMod || a.ValueType != vt || a.ValueIsReference != (vt == 7 || vt == 9) {
			t.Errorf("vt %d: %+v", vt, a)
		}
		if a.CompanyID != "0123" || a.FreeField != rest[4:] {
			t.Errorf("vt %d: company/free = %q/%q", vt, a.CompanyID, a.FreeField)
		}
	}
}

func TestArrecadacaoCNPJSegment(t *testing.T) {
	rest := "11222333" + strings.Repeat("9", 21)
	b, err := NewArrecadacao(SegmentCNPJ, 8, 5000, rest)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b.Formatted())
	if err != nil {
		t.Fatal(err)
	}
	a := p.Arrecadacao
	if a.CNPJBase != "11222333" || a.CompanyID != "" || a.FreeField != strings.Repeat("9", 21) {
		t.Errorf("unexpected %+v", a)
	}
}

func TestArrecadacaoCheckDigitErrors(t *testing.T) {
	b, err := NewArrecadacao(SegmentTelecom, 8, 98765, strings.Repeat("7", 29))
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(b.DigitableLine)
	for block := 1; block <= 4; block++ {
		l := append([]byte(nil), line...)
		i := (block-1)*12 + 11 // the block's DV
		l[i] = '0' + (l[i]-'0'+1)%10
		var cde *CheckDigitError
		if err := Validate(string(l)); !errors.As(err, &cde) || cde.Field != block {
			t.Errorf("block %d: err = %v", block, err)
		}
	}
	code := []byte(b.Barcode)
	code[3] = '0' + (code[3]-'0'+1)%10
	var cde *CheckDigitError
	if err := Validate(string(code)); !errors.As(err, &cde) || cde.Field != FieldGeneral {
		t.Errorf("general DV: err = %v", err)
	}
}

func TestNewArrecadacaoErrors(t *testing.T) {
	rest := strings.Repeat("0", 29)
	tests := []struct {
		seg   Segment
		vt    int
		value int64
		rest  string
		want  error
	}{
		{0, 6, 0, rest, ErrSegment},
		{8, 6, 0, rest, ErrSegment},
		{SegmentBanco, 5, 0, rest, ErrValueType},
		{SegmentBanco, 6, -1, rest, ErrAmount},
		{SegmentBanco, 6, maxArrecadacaoAmount + 1, rest, ErrAmount},
		{SegmentBanco, 6, 0, "1", ErrFreeField},
		{SegmentBanco, 6, 0, strings.Repeat("a", 29), ErrFreeField},
	}
	for _, tt := range tests {
		if _, err := NewArrecadacao(tt.seg, tt.vt, tt.value, tt.rest); !errors.Is(err, tt.want) {
			t.Errorf("NewArrecadacao(%d,%d,%d,%q) = %v, want %v", tt.seg, tt.vt, tt.value, tt.rest, err, tt.want)
		}
	}
}

func TestSegmentNames(t *testing.T) {
	for s := Segment(0); s <= 10; s++ {
		defined := s >= 1 && s <= 7 || s == 9
		if (s.String() != "") != defined {
			t.Errorf("Segment(%d).String() = %q", s, s.String())
		}
	}
}

func TestArrecadacaoLineInvalidValueType(t *testing.T) {
	if err := Validate("805" + strings.Repeat("0", 45)); !errors.Is(err, ErrValueType) {
		t.Errorf("err = %v", err)
	}
}
