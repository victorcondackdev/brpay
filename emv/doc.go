// Package emv implements the EMVCo Merchant-Presented Mode (MPM) QR code
// payload format, the TLV encoding that underlies the Brazilian Pix BR Code.
//
// A payload is a sequence of data objects, each written as a 2-digit ID, a
// 2-digit decimal length and the value itself:
//
//	000201 26580014br.gov.bcb.pix0136... 6304 1D3D
//	ID=00 len=02 value="01"
//
// Some IDs are templates whose value is itself a TLV list: merchant account
// information (26–51), additional data (62), merchant information in an
// alternate language (64) and the unreserved templates (80–99). [Parse]
// expands those one level deep into [Field.Children] while keeping the raw
// value, so a payload can be inspected and re-encoded without losing data.
//
// The package is split by concern:
//
//   - [Parse] and [ParseTemplate] check TLV syntax only.
//   - [Validate] checks payload structure (00 = "01" first, 63 last).
//   - [VerifyCRC] checks integrity (CRC-16/CCITT-FALSE in field 63).
//
// Errors are positional: a [*ParseError] reports the byte offset and field ID
// where decoding stopped, e.g. "emv: byte 57, field 59: length 30 exceeds
// remaining 12", which makes it suitable for diagnostics and linters.
//
// Lengths and offsets are counted in bytes. EMV payloads are restricted to
// printable ASCII, where bytes and characters coincide.
package emv
