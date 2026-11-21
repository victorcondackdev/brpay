// Package brpay is the single entry point for whatever a user pasted into a
// payment field: a Pix "copia e cola" payload, a boleto barcode or linha
// digitável, a CPF or CNPJ, a Pix key or an EndToEndId.
//
// [Detect] classifies the input by shape and [Decode] hands it to the
// matching package, returning a [Result] that also carries diagnostics:
// lint issues for Pix payloads (even ones that fail to decode) and
// corrected lines for boletos with a single mistyped digit.
//
// The format-specific packages can be used on their own:
//
//   - [github.com/victorcondackdev/brpay/pix]: BR Code decode, build, lint;
//     Pix keys; EndToEndIds.
//   - [github.com/victorcondackdev/brpay/boleto]: barcodes and linhas
//     digitáveis, bancário and arrecadação.
//   - [github.com/victorcondackdev/brpay/document]: CPF and CNPJ, including
//     the alphanumeric CNPJ.
//   - [github.com/victorcondackdev/brpay/emv]: the EMV MPM TLV layer under
//     the BR Code.
//
// Amounts are int64 cents throughout; no floating point is involved in
// parsing or building codes.
package brpay
