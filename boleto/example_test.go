package boleto_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/victorcondackdev/brpay/boleto"
)

func ExampleParse() {
	b, err := boleto.Parse("00190.00009 00000.000000 00000.000075 9 10000000010000")
	if err != nil {
		panic(err)
	}
	due, _ := b.Bancario.DueDateAt(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	fmt.Println(b.Type, b.Bancario.BankName)
	fmt.Println(b.Barcode)
	fmt.Printf("R$ %d,%02d due %s\n", b.AmountCents/100, b.AmountCents%100, due.Format(time.DateOnly))
	// Output:
	// bancário Banco do Brasil
	// 00199100000000100000000000000000000000000007
	// R$ 100,00 due 2025-02-22
}

func ExampleValidate() {
	err := boleto.Validate("00190.00009 00000.000001 00000.000075 9 10000000010000")
	var cde *boleto.CheckDigitError
	if errors.As(err, &cde) {
		fmt.Printf("check field %d: expected %c, got %c\n", cde.Field, cde.Expected, cde.Got)
	}
	// Output:
	// check field 2: expected 0, got 1
}

func ExampleNewBancario() {
	due := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	b, err := boleto.NewBancario("341", due, 15990, "1090100000012345678900001")
	if err != nil {
		panic(err)
	}
	fmt.Println(b.Formatted())
	fmt.Println(b.Bancario.Factor)
	// Output:
	// 34191.09016 00000.012344 56789.000017 9 16560000015990
	// 1656
}

func ExampleBarcodeToLine() {
	line, err := boleto.BarcodeToLine("82650000000100000010000000000000000000000000")
	if err != nil {
		panic(err)
	}
	fmt.Println(line)
	// Output:
	// 826500000003100000010007000000000000000000000000
}

func ExampleLineToBarcode() {
	code, err := boleto.LineToBarcode("82650000000-3 10000001000-7 00000000000-0 00000000000-0")
	if err != nil {
		panic(err)
	}
	fmt.Println(code)
	// Output:
	// 82650000000100000010000000000000000000000000
}

func ExampleDueDateAt() {
	// Factor 1000 means 2000-07-03 in the first cycle and 2025-02-22 after
	// the February 2025 rollover; the reference date picks the cycle.
	for _, ref := range []string{"2001-01-01", "2026-01-01"} {
		r, _ := time.Parse(time.DateOnly, ref)
		d, _ := boleto.DueDateAt(1000, r)
		fmt.Println(ref, "->", d.Format(time.DateOnly))
	}
	// Output:
	// 2001-01-01 -> 2000-07-03
	// 2026-01-01 -> 2025-02-22
}

func ExampleFactorFor() {
	for _, s := range []string{"2025-02-21", "2025-02-22"} {
		d, _ := time.Parse(time.DateOnly, s)
		f, _ := boleto.FactorFor(d)
		fmt.Println(s, f)
	}
	// Output:
	// 2025-02-21 9999
	// 2025-02-22 1000
}

func ExampleSuggestFix() {
	// The user typed 5 instead of 0 in the second field. Check digits
	// narrow the fix down to two candidates; the second is the original.
	typo := "00190.00009 00000.500000 00000.000075 9 10000000010000"
	for _, s := range boleto.SuggestFix(typo) {
		b, _ := boleto.Parse(s)
		fmt.Println(b.Formatted())
	}
	// Output:
	// 00190.00009 00009.500000 00000.000075 9 10000000010000
	// 00190.00009 00000.000000 00000.000075 9 10000000010000
}

func ExampleParseHybrid() {
	pix := "00020101021226580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"
	h, err := boleto.ParseHybrid("00199100000000100000000000000000000000000007", pix)
	if err != nil {
		panic(err)
	}
	fmt.Println(h.Boleto.Bancario.BankCode, strings.HasPrefix(h.PixPayload, "000201"))
	// Output:
	// 001 true
}

func ExampleNewArrecadacao() {
	b, err := boleto.NewArrecadacao(boleto.SegmentEnergiaGas, 8, 25075, "0123"+strings.Repeat("0", 25))
	if err != nil {
		panic(err)
	}
	fmt.Println(b.Formatted())
	fmt.Println(b.Arrecadacao.Segment, b.Arrecadacao.Modulus)
	// Output:
	// 83810000002-7 50750123000-8 00000000000-0 00000000000-0
	// Energia elétrica e gás 11
}
