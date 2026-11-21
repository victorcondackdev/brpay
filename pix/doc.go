// Package pix decodes, generates and lints Pix BR Codes ("copia e cola"
// payloads), validates Pix keys and handles EndToEndIds, following the
// Banco Central do Brasil specifications.
//
// The BR Code is an EMVCo Merchant-Presented Mode payload (see package
// [github.com/victorcondackdev/brpay/emv]) with a merchant account template
// whose GUI is "br.gov.bcb.pix":
//
//   - [Decode] parses a payload into a [BRCode], verifying its CRC.
//   - [NewStatic] and [NewDynamic] build payloads from options.
//   - [Lint] reports every problem it can find in a payload, as a list of
//     [Issue] values, without stopping at the first one. It targets the
//     mistakes that make banking apps reject a code: a stale CRC, an amount
//     written with a comma, a merchant name that is too long, accents, stray
//     whitespace from copy-paste and so on.
//   - [NormalizeKey] and [DetectKey] validate the five Pix key types.
//   - [ParseEndToEndID] and [NewEndToEndID] handle transaction identifiers.
//
// Decode is deliberately lenient about content it does not need to
// interpret (an unrecognized key format, unknown templates) and preserves
// every field in [BRCode.Fields], so codes for products introduced by the
// BCB after this package was written are not lost. Use Lint for strict
// checking.
package pix
