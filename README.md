# brpay

[![CI](https://github.com/victorcondackdev/brpay/actions/workflows/ci.yml/badge.svg)](https://github.com/victorcondackdev/brpay/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/victorcondackdev/brpay.svg)](https://pkg.go.dev/github.com/victorcondackdev/brpay)
[![Go Report Card](https://goreportcard.com/badge/github.com/victorcondackdev/brpay)](https://goreportcard.com/report/github.com/victorcondackdev/brpay)

brpay is a dependency-free Go library for the codes Brazilian users paste into
payment fields: Pix "copia e cola" (BR Code), boleto barcodes and linhas
digitáveis, CPF/CNPJ, Pix keys and EndToEndIds. It decodes, validates and
builds them, and when the input is wrong it says where and why: positional
parse errors, a linter for Pix payloads and typo recovery for linhas
digitáveis. One call, `brpay.Decode`, figures out what was pasted.

**[Try it in the browser](https://victorcondackdev.github.io/brpay/)**: the
playground is this library compiled to WebAssembly and runs fully offline.

## Why

- **Zero dependencies.** Standard library only, Go 1.22+.
- **No panics on any input.** Every parser is fuzzed; malformed input is an
  error, never a crash.
- **Alphanumeric CNPJ ready.** The format the Receita Federal issues from
  July 2026 is validated, formatted and accepted as a Pix key.
- **2025 boleto factor reset handled.** The due-date factor wrapped from 9999
  to 1000 on 2025-02-22; dates are resolved against a reference window
  instead of silently landing in 2000.
- **Positional diagnostics.** Errors carry the byte offset, field ID or
  boleto field that failed, and work with `errors.Is` / `errors.As`.
- **Linter for copia e cola.** `pix.Lint` reports every problem at once:
  stale CRC, comma in the amount, accents, oversized fields, stray
  whitespace from copy-paste.
- **Typo recovery for linhas digitáveis.** Check digits locate the mistyped
  field and `boleto.SuggestFix` proposes the corrected lines.
- **[WASM playground](https://victorcondackdev.github.io/brpay/)** and a
  CLI built on the same code.

## Install

```sh
go get github.com/victorcondackdev/brpay
```

CLI:

```sh
go install github.com/victorcondackdev/brpay/cmd/brpay@latest
```

## Quick start

Decode whatever the user pasted:

```go
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
// pix Fulano de Tal EVP 123e4567-e12b-12d1-a456-426655440000
// boleto invalid: boleto: check digit mismatch in field 2: expected 9, got 0
//    did you mean 00190000090000950000000000000075910000000010000
//    did you mean 00190000090000000000000000000075910000000010000
```

Build a static Pix code:

```go
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
// 00020126360014br.gov.bcb.pix0114+5521987654321520400005303986540512.505802BR5918Padaria Pao Quente6007Niteroi62120508PEDIDO4263048559
// 0 issues
```

Parse a boleto and inspect a check-digit failure:

```go
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
// Itaú Unibanco 15990 2026-12-10
// 34199165600000159901090100000012345678900001
// field 2 check digit should be 4
```

Validate documents:

```go
fmt.Println(document.ValidateCNPJ("12.ABC.345/01DE-35"))
fmt.Println(errors.Is(document.ValidateCPF("529.982.247-24"), document.ErrCheckDigit))
fmt.Println(document.Format("52998224725"), document.Mask("52998224725"))
// <nil>
// true
// 529.982.247-25 ***.982.247-**
```

These snippets are compiled and their output checked by
[`readme_example_test.go`](readme_example_test.go).

## Packages

| Package | Purpose |
| --- | --- |
| [`brpay`](https://pkg.go.dev/github.com/victorcondackdev/brpay) | `Detect` / `Decode`: one entry point for anything pasted, with diagnostics |
| [`pix`](https://pkg.go.dev/github.com/victorcondackdev/brpay/pix) | BR Code decode, static and dynamic builders, `Lint`, Pix keys, EndToEndId |
| [`boleto`](https://pkg.go.dev/github.com/victorcondackdev/brpay/boleto) | Barcode and linha digitável (bancário, arrecadação), due-date factor, `SuggestFix`, hybrid boletos |
| [`document`](https://pkg.go.dev/github.com/victorcondackdev/brpay/document) | CPF and CNPJ (numeric and alphanumeric): validate, format, mask |
| [`emv`](https://pkg.go.dev/github.com/victorcondackdev/brpay/emv) | EMV MPM TLV parse, encode and validate; CRC16 |
| [`cmd/brpay`](cmd/brpay) | Command-line interface |

## CLI

```text
$ brpay help
brpay decodes, lints and builds Brazilian payment codes.

Usage:
  brpay decode [--json] [input | -]
        Detect and decode a Pix copia e cola, boleto barcode or linha
        digitável, CPF/CNPJ, Pix key or EndToEndId.
  brpay lint [--json] [payload | -]
        Report every problem in a Pix copia e cola payload.
  brpay pix static [--json] --key KEY --name NAME --city CITY
                   [--amount 10.50] [--txid ID] [--desc TEXT]
        Build a static Pix copia e cola payload.
  brpay boleto fix [--json] [linha | -]
        Suggest corrections for a linha digitável with one mistyped digit.
  brpay version
        Print the version.

Input is read from standard input when it is omitted or "-". Several
arguments are joined with spaces, so a linha digitável can be pasted
unquoted.

Exit status: 0 success, 1 invalid input, 2 usage error.
```

```text
$ brpay decode 00190.00009 00000.000000 00000.000075 9 10000000010000
kind            boleto
type            bancario
formatted       00190.00009 00000.000000 00000.000075 9 10000000010000
barcode         00199100000000100000000000000000000000000007
digitable line  00190000090000000000000000000075910000000010000
bank            001 Banco do Brasil
amount          R$ 100,00
due date        2025-02-22
factor          1000
free field      0000000000000000000000007

$ brpay pix static --key fulano@example.com --name "Fulano de Tal" --city Recife --amount 49.90 | brpay decode --json
```

`--json` output uses stable snake_case field names and is the same document
the WebAssembly build returns.

## Design notes

- **Money is `int64` cents.** Amounts are parsed from and formatted to their
  decimal string form digit by digit; no `float64` is ever involved.
- **Errors are values you can inspect.** Each package exports sentinel errors
  (`pix.ErrInvalidKey`, `boleto.ErrCheckDigit`, `document.ErrCheckDigit`,
  `emv.ErrCRC`, ...) and structured ones (`*emv.ParseError` with byte offset
  and field ID, `*boleto.CheckDigitError` with field and digits).
  `brpay.Decode` returns them unwrapped from the underlying package.
- **Strict where it matters, lenient where it does not.** Check digits and
  CRCs are always verified. `pix.Decode` accepts unknown templates and
  key formats it cannot interpret, keeping every field, so codes for future
  BCB products are not lost; `pix.Lint` is the strict counterpart. Builders
  (`pix.NewStatic`, `boleto.NewBancario`) only emit canonical output.
- **Ambiguity is resolved explicitly.** `brpay.Detect` documents its order:
  a valid boleto wins over everything, CPF/CNPJ are reported as documents
  even though they are also Pix keys, and national phone numbers without
  `+55` are not keys because they cannot be told apart from a CPF.
- **Deterministic and offline.** No network access, no global state; the
  current time is only used where a function says so (`DueDate`), with an
  explicit-reference variant next to it (`DueDateAt`).

## Roadmap

- Pix Automático (recurring Pix) templates in `pix`.
- ISO 20022 helpers for `pacs.008` / `pacs.002` messages used by the SPI.
- Pix key validation parity with the DICT rules.
- More banks in the boleto bank-name table.

## Disclaimer

brpay is an independent project, not affiliated with or endorsed by the
Banco Central do Brasil or FEBRABAN. It follows the public specifications,
but payment rules change and PSPs differ; verify behavior with your PSP
before relying on it in production. Provided under the [MIT License](LICENSE)
without warranty.
