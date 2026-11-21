package brpay_test

// The examples in this file are the code snippets of README.md, kept here
// so that they are compiled and their output is checked.

import (
	"errors"
	"fmt"

	"github.com/victorcondackdev/brpay"
	"github.com/victorcondackdev/brpay/boleto"
	"github.com/victorcondackdev/brpay/document"
	"github.com/victorcondackdev/brpay/pix"
)

func Example_readmeDecode() {
	for _, input := range []string{
		"00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
			"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D",
		"00190.00009 00000.500000 00000.000075 9 10000000010000", // one digit mistyped
	} {
		r, err := brpay.Decode(input)
		if err != nil {
			fmt.Println(r.Kind, "invalid:", err)
			for _, issue := range r.Issues {
				fmt.Println("  ", issue)
			}
			for _, fix := range r.Suggestions {
				fmt.Println("   did you mean", fix)
			}
			continue
		}
		switch r.Kind {
		case brpay.KindPix:
			fmt.Println(r.Kind, r.Pix.MerchantName, r.Pix.KeyType, r.Pix.Key)
		case brpay.KindBoleto:
			fmt.Println(r.Kind, r.Boleto.Formatted(), r.Boleto.AmountCents)
		}
	}
	// Output:
	// pix Fulano de Tal EVP 123e4567-e12b-12d1-a456-426655440000
	// boleto invalid: boleto: check digit mismatch in field 2: expected 9, got 0
	//    did you mean 00190000090000950000000000000075910000000010000
	//    did you mean 00190000090000000000000000000075910000000010000
}

func Example_readmePix() {
	payload, err := pix.NewStatic(pix.StaticOptions{
		Key:          "+55 (21) 98765-4321", // normalized to +5521987654321
		MerchantName: "Padaria Pão Quente",  // transliterated to ASCII
		MerchantCity: "Niterói",
		Cents:        1250, // R$ 12,50: money is always int64 cents
		TxID:         "PEDIDO42",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(payload)
	fmt.Println(len(pix.Lint(payload)), "issues")
	// Output:
	// 00020126360014br.gov.bcb.pix0114+5521987654321520400005303986540512.505802BR5918Padaria Pao Quente6007Niteroi62120508PEDIDO4263048559
	// 0 issues
}

func Example_readmeBoleto() {
	b, err := boleto.Parse("34191.09016 00000.012344 56789.000017 9 16560000015990")
	if err != nil {
		panic(err)
	}
	due, _ := b.Bancario.DueDate() // factor 1656, after the 2025 reset
	fmt.Println(b.Bancario.BankName, b.AmountCents, due.Format("2006-01-02"))
	fmt.Println(b.Barcode)

	err = boleto.Validate("34191.09016 00000.012345 56789.000017 9 16560000015990")
	var cde *boleto.CheckDigitError
	if errors.As(err, &cde) {
		fmt.Println("field", cde.Field, "check digit should be", string(cde.Expected))
	}
	// Output:
	// Itaú Unibanco 15990 2026-12-10
	// 34199165600000159901090100000012345678900001
	// field 2 check digit should be 4
}

func Example_readmeDocument() {
	fmt.Println(document.ValidateCNPJ("12.ABC.345/01DE-35"))
	fmt.Println(errors.Is(document.ValidateCPF("529.982.247-24"), document.ErrCheckDigit))
	fmt.Println(document.Format("52998224725"), document.Mask("52998224725"))
	// Output:
	// <nil>
	// true
	// 529.982.247-25 ***.982.247-**
}
