package boleto

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Hand-computed bancário vector.
//
// Bank 001, currency 9, factor 1000, amount 0000010000 (R$ 100,00), free
// field of 24 zeros followed by 7. The 43 digits without the DAC are
//
//	0 0 1 9 | 1 0 0 0 | 0 0 0 0 0 1 0 0 0 0 | 0…0 7
//
// Mod 11, weights 2..9 from the right; only non-zero digits matter:
//
//	7 (pos from right 0, w 2) = 14
//	1 in amount (pos 29, w 7) =  7
//	1 in factor (pos 38, w 8) =  8
//	9 currency  (pos 39, w 9) = 81
//	1 in bank   (pos 40, w 2) =  2
//	sum = 112, 112 % 11 = 2, DAC = 11 − 2 = 9
//
// Field 1 = "001" "9" "00000": products from the right 0,0,0,0,0, 9×1, 1×2
// → sum 11 → DV 9. Field 2 is all zeros → DV 0. Field 3 = "0000000007":
// 7×2 = 14 → 1+4 = 5 → DV 5.
const (
	handBarcode   = "0019" + "9" + "1000" + "0000010000" + "000000000000000000000000" + "7"
	handLine      = "0019000009" + "00000000000" + "00000000075" + "9" + "10000000010000"
	handFormatted = "00190.00009 00000.000000 00000.000075 9 10000000010000"
)

func TestHandComputedBancario(t *testing.T) {
	b, err := Parse(handBarcode)
	if err != nil {
		t.Fatal(err)
	}
	if b.DigitableLine != handLine {
		t.Errorf("line = %s, want %s", b.DigitableLine, handLine)
	}
	if b.Formatted() != handFormatted || b.String() != handFormatted {
		t.Errorf("formatted = %q", b.Formatted())
	}
	if b.Type != TypeBancario || b.AmountCents != 10000 {
		t.Errorf("type/amount = %v/%d", b.Type, b.AmountCents)
	}
	bk := b.Bancario
	if bk.BankCode != "001" || bk.BankName != "Banco do Brasil" || bk.Currency != 9 || bk.Factor != 1000 {
		t.Errorf("fields = %+v", bk)
	}
	built, err := NewBancario("001", time.Date(2000, 7, 3, 0, 0, 0, 0, time.UTC), 10000, strings.Repeat("0", 24)+"7")
	if err != nil {
		t.Fatal(err)
	}
	if built.Barcode != handBarcode {
		t.Errorf("NewBancario barcode = %s, want %s", built.Barcode, handBarcode)
	}
	if built.Bancario.Factor != 1000 {
		t.Errorf("factor = %d", built.Bancario.Factor)
	}
}

func TestBancarioRoundTrip(t *testing.T) {
	due := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		bank   string
		due    time.Time
		amount int64
		free   string
	}{
		{"341", due, 123456, "1090100000012345678900001"},
		{"237", time.Time{}, 0, "9999999999999999999999999"},
		{"104", time.Date(2025, 2, 21, 0, 0, 0, 0, time.UTC), maxBancarioAmount, "0123456789012345678901234"},
		{"999", time.Date(2049, 10, 13, 0, 0, 0, 0, time.UTC), 1, "5555500000111112222233333"},
	}
	for _, tt := range tests {
		b, err := NewBancario(tt.bank, tt.due, tt.amount, tt.free)
		if err != nil {
			t.Fatalf("NewBancario(%s): %v", tt.bank, err)
		}
		line, err := BarcodeToLine(b.Barcode)
		if err != nil || line != b.DigitableLine {
			t.Fatalf("BarcodeToLine = %s, %v", line, err)
		}
		code, err := LineToBarcode(b.Formatted())
		if err != nil || code != b.Barcode {
			t.Fatalf("LineToBarcode = %s, %v", code, err)
		}
		for _, in := range []string{b.Barcode, b.DigitableLine, b.Formatted()} {
			p, err := Parse(in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", in, err)
			}
			if p.Barcode != b.Barcode || p.AmountCents != tt.amount || p.Bancario.FreeField != tt.free || p.Bancario.BankCode != tt.bank {
				t.Errorf("Parse(%q) = %+v %+v", in, p, p.Bancario)
			}
			got, ok := p.Bancario.DueDateAt(tt.due)
			if tt.due.IsZero() {
				if ok || p.Bancario.Factor != 0 {
					t.Errorf("expected no due date, got %v", got)
				}
			} else if !ok || !got.Equal(tt.due) {
				t.Errorf("DueDateAt = %v %v, want %v", got, ok, tt.due)
			}
		}
	}
}

func TestBancarioUnknownBank(t *testing.T) {
	b, err := NewBancario("999", time.Time{}, 1, strings.Repeat("1", 25))
	if err != nil {
		t.Fatal(err)
	}
	if b.Bancario.BankName != "" {
		t.Errorf("BankName = %q", b.Bancario.BankName)
	}
	if _, ok := b.Bancario.DueDate(); ok {
		t.Error("factor 0 must have no due date")
	}
}

func TestNewBancarioErrors(t *testing.T) {
	free := strings.Repeat("0", 25)
	tests := []struct {
		bank   string
		due    time.Time
		amount int64
		free   string
		want   error
	}{
		{"01", time.Time{}, 0, free, ErrBankCode},
		{"0a1", time.Time{}, 0, free, ErrBankCode},
		{"800", time.Time{}, 0, free, ErrProductID},
		{"001", time.Time{}, -1, free, ErrAmount},
		{"001", time.Time{}, maxBancarioAmount + 1, free, ErrAmount},
		{"001", time.Time{}, 0, "123", ErrFreeField},
		{"001", time.Time{}, 0, strings.Repeat("x", 25), ErrFreeField},
		{"001", time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), 0, free, ErrDueDate},
	}
	for _, tt := range tests {
		if _, err := NewBancario(tt.bank, tt.due, tt.amount, tt.free); !errors.Is(err, tt.want) {
			t.Errorf("NewBancario(%q, %v, %d, %q) = %v, want %v", tt.bank, tt.due, tt.amount, tt.free, err, tt.want)
		}
	}
}

func TestBancarioCheckDigitErrors(t *testing.T) {
	flip := func(s string, i int) string {
		b := []byte(s)
		b[i] = '0' + (b[i]-'0'+1)%10
		return string(b)
	}
	tests := []struct {
		name  string
		in    string
		field int
	}{
		{"field 1 data", flip(handLine, 2), 1},
		{"field 1 dv", flip(handLine, 9), 1},
		{"field 2", flip(handLine, 15), 2},
		{"field 3", flip(handLine, 25), 3},
		{"field 4 (DAC)", flip(handLine, 32), FieldGeneral},
		{"field 5", flip(handLine, 40), FieldGeneral},
		{"barcode DAC", flip(handBarcode, 4), FieldGeneral},
		{"barcode data", flip(handBarcode, 30), FieldGeneral},
	}
	for _, tt := range tests {
		err := Validate(tt.in)
		var cde *CheckDigitError
		if !errors.As(err, &cde) || !errors.Is(err, ErrCheckDigit) {
			t.Fatalf("%s: err = %v", tt.name, err)
		}
		if cde.Field != tt.field {
			t.Errorf("%s: field = %d, want %d", tt.name, cde.Field, tt.field)
		}
		if cde.Expected == cde.Got || cde.Error() == "" {
			t.Errorf("%s: bad error %+v", tt.name, cde)
		}
	}
}

func TestCheckDigitErrorMessage(t *testing.T) {
	e := &CheckDigitError{Field: 2, Expected: '3', Got: '4'}
	if got, want := e.Error(), "boleto: check digit mismatch in field 2: expected 3, got 4"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	e = &CheckDigitError{Field: FieldGeneral, Expected: '3', Got: '4'}
	if got, want := e.Error(), "boleto: general check digit mismatch: expected 3, got 4"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"", ErrLength},
		{"123", ErrLength},
		{strings.Repeat("1", 49), ErrLength},
		{strings.Repeat("1", 46), ErrLength},
		{handLine[:10] + "x" + handLine[11:], ErrCharacters},
		{handLine + "é", ErrCharacters},
		{"8" + handLine[1:], ErrProductID},
		{"810" + strings.Repeat("0", 41), ErrValueType},
		{"810" + strings.Repeat("0", 45), ErrValueType},
	}
	for _, tt := range tests {
		if err := Validate(tt.in); !errors.Is(err, tt.want) {
			t.Errorf("Validate(%q) = %v, want %v", tt.in, err, tt.want)
		}
	}
}

func TestConversionErrors(t *testing.T) {
	if _, err := BarcodeToLine(handLine); !errors.Is(err, ErrLength) {
		t.Errorf("BarcodeToLine(line) = %v", err)
	}
	if _, err := BarcodeToLine("x"); !errors.Is(err, ErrCharacters) {
		t.Errorf("BarcodeToLine(x) = %v", err)
	}
	if _, err := BarcodeToLine(handBarcode[:4] + "0" + handBarcode[5:]); !errors.Is(err, ErrCheckDigit) {
		t.Errorf("BarcodeToLine(bad DAC) = %v", err)
	}
	if _, err := LineToBarcode(handBarcode); !errors.Is(err, ErrLength) {
		t.Errorf("LineToBarcode(barcode) = %v", err)
	}
	if _, err := LineToBarcode("x"); !errors.Is(err, ErrCharacters) {
		t.Errorf("LineToBarcode(x) = %v", err)
	}
	if _, err := LineToBarcode(handLine[:9] + "0" + handLine[10:]); !errors.Is(err, ErrCheckDigit) {
		t.Errorf("LineToBarcode(bad DV) = %v", err)
	}
	if _, err := LineToBarcode(strings.Repeat("0", 48)); !errors.Is(err, ErrProductID) {
		t.Errorf("LineToBarcode(48 zeros) = %v", err)
	}
}

func TestTypeString(t *testing.T) {
	if TypeBancario.String() != "bancário" || TypeArrecadacao.String() != "arrecadação" || TypeUnknown.String() != "unknown" {
		t.Error("Type.String mismatch")
	}
	if formatLine("123") != "123" {
		t.Error("formatLine passthrough")
	}
}
