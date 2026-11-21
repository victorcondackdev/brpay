package pix_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/victorcondackdev/brpay/pix"
)

func Example() {
	payload, err := pix.NewStatic(pix.StaticOptions{
		Key:          "+55 (21) 98765-4321",
		MerchantName: "Padaria Pão Quente",
		MerchantCity: "Niterói",
		Cents:        1250,
		TxID:         "PEDIDO42",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(payload)

	code, err := pix.Decode(payload)
	if err != nil {
		panic(err)
	}
	fmt.Println(code.KeyType, code.Key, code.Amount, code.MerchantName, code.MerchantCity, code.TxID)
	// Output:
	// 00020126360014br.gov.bcb.pix0114+5521987654321520400005303986540512.505802BR5918Padaria Pao Quente6007Niteroi62120508PEDIDO4263048559
	// PHONE +5521987654321 12.50 Padaria Pao Quente Niteroi PEDIDO42
}

func ExampleDecode() {
	code, err := pix.Decode("00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D")
	if err != nil {
		panic(err)
	}
	fmt.Println("static:", code.Static)
	fmt.Println("key:", code.Key, code.KeyType)
	fmt.Println("merchant:", code.MerchantName, "-", code.MerchantCity)
	fmt.Println("open amount:", code.Amount == "")
	// Output:
	// static: true
	// key: 123e4567-e12b-12d1-a456-426655440000 EVP
	// merchant: Fulano de Tal - BRASILIA
	// open amount: true
}

func ExampleLint() {
	// A payload someone edited by hand: comma in the amount, accented city,
	// and a CRC that no longer matches.
	payload := "00020126330014br.gov.bcb.pix0111529982247255204000053039865405" +
		"10,005802BR5906Fulano6008Niterói62070503***63041D3D\n"
	for _, issue := range pix.Lint(payload) {
		fmt.Println(issue)
	}
	// Output:
	// warning: payload has surrounding whitespace or line breaks (often a copy-paste artifact); some apps reject it
	// error [63]: CRC is "1D3D", expected 24A3 (was the payload edited after the CRC was computed?)
	// error [60]: non-ASCII or control character 'ó' at byte 5 of the value; transliterate accents ("São" to "Sao")
	// error [54]: amount "10,00": uses a comma; the decimal separator must be a dot
}

func ExampleNormalizeKey() {
	for _, k := range []string{"529.982.247-25", "Fulano@Example.com", "+55 (21) 98765-4321", "12.ABC.345/01DE-35"} {
		key, typ, err := pix.NormalizeKey(k)
		fmt.Println(typ, key, err)
	}
	// Output:
	// CPF 52998224725 <nil>
	// EMAIL fulano@example.com <nil>
	// PHONE +5521987654321 <nil>
	// CNPJ 12ABC34501DE35 <nil>
}

func ExampleParseEndToEndID() {
	id, err := pix.ParseEndToEndID("E60701190202611211430abcdeFGHIJ1")
	if err != nil {
		panic(err)
	}
	fmt.Println(id.ISPB, id.Time.Format(time.RFC3339), id.Sequence)
	// Output: 60701190 2026-11-21T14:30:00Z abcdeFGHIJ1
}

func ExampleNewEndToEndID() {
	at := time.Date(2026, 11, 21, 14, 30, 0, 0, time.UTC)
	// A nil reader uses crypto/rand; a fixed one makes the output reproducible.
	id, err := pix.NewEndToEndID("60701190", at, strings.NewReader(strings.Repeat("pix-brcode", 4)))
	if err != nil {
		panic(err)
	}
	fmt.Println(id, len(id))
	// Output: E60701190202611211430ohwjaqbncdo 32
}
