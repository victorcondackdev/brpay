package brpay_test

import (
	"errors"
	"fmt"

	"github.com/victorcondackdev/brpay"
	"github.com/victorcondackdev/brpay/boleto"
)

func Example() {
	inputs := []string{
		"00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
			"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D",
		"00190.00009 00000.000000 00000.000075 9 10000000010000",
		"12.ABC.345/01DE-35",
		"fulano@example.com",
	}
	for _, in := range inputs {
		r, err := brpay.Decode(in)
		if err != nil {
			fmt.Println(r.Kind, "error:", err)
			continue
		}
		switch r.Kind {
		case brpay.KindPix:
			fmt.Println(r.Kind, r.Pix.KeyType, r.Pix.MerchantName)
		case brpay.KindBoleto:
			fmt.Println(r.Kind, r.Boleto.Bancario.BankName, r.Boleto.AmountCents)
		case brpay.KindDocument:
			fmt.Println(r.Kind, r.Document.Kind, r.Document.Formatted, r.Document.Alphanumeric)
		case brpay.KindPixKey:
			fmt.Println(r.Kind, r.PixKey.Type, r.PixKey.Key)
		}
	}
	// Output:
	// pix EVP Fulano de Tal
	// boleto Banco do Brasil 10000
	// document CNPJ 12.ABC.345/01DE-35 true
	// pix_key EMAIL fulano@example.com
}

func ExampleDecode_diagnostics() {
	// One digit mistyped in the second field of a linha digitável.
	r, err := brpay.Decode("00190.00009 00000.500000 00000.000075 9 10000000010000")
	var cde *boleto.CheckDigitError
	if errors.As(err, &cde) {
		fmt.Println("check digit of field", cde.Field, "does not match")
	}
	for _, s := range r.Suggestions {
		fmt.Println("did you mean", s)
	}

	// A copia-e-cola edited by hand: Lint explains what is wrong.
	r, _ = brpay.Decode("00020126330014br.gov.bcb.pix011152998224725520400005303986" +
		"540510,005802BR5906Fulano6006Recife62070503***63041D3D")
	for _, issue := range r.Issues {
		fmt.Println(issue)
	}
	// Output:
	// check digit of field 2 does not match
	// did you mean 00190000090000950000000000000075910000000010000
	// did you mean 00190000090000000000000000000075910000000010000
	// error [63]: CRC is "1D3D", expected F3F2 (was the payload edited after the CRC was computed?)
	// error [54]: amount "10,00": uses a comma; the decimal separator must be a dot
}

func ExampleDetect() {
	for _, in := range []string{"529.982.247-25", "+5521987654321", "E60701190202611211430abcdeFGHIJ1", "hello"} {
		fmt.Println(brpay.Detect(in))
	}
	// Output:
	// document
	// pix_key
	// end_to_end_id
	// unknown
}
