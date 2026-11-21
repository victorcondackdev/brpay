// Package boleto parses, validates, converts and builds Brazilian boleto
// codes following the FEBRABAN layouts.
//
// Two families share the 44-digit barcode size but differ in everything else:
//
//   - Bancário ("boleto de cobrança", bank slips): 44-digit barcode and a
//     47-digit linha digitável split in five fields. Fields 1 to 3 carry a
//     modulo-10 check digit, field 4 is the general modulo-11 check digit
//     (DAC) and field 5 holds the due-date factor and the amount.
//   - Arrecadação / convênio (utility bills, taxes, fines): barcodes start
//     with 8 and the 48-digit linha digitável is four blocks of eleven
//     barcode digits, each followed by its own check digit. The modulus
//     (10 or 11) is chosen by the barcode's third digit.
//
// [Parse] accepts any of the three forms (44, 47 or 48 digits), with or
// without the usual spaces, dots and hyphens, detects the family and checks
// every check digit. Failures are reported as a [*CheckDigitError] that names
// the field the user most likely mistyped, and [SuggestFix] goes one step
// further by proposing corrected lines for a single-digit typo.
//
// # Due-date factor rollover
//
// The bancário due date is stored as a 4-digit factor: the number of days
// since 1997-10-07. Factor 9999 was reached on 2025-02-21 and, as defined by
// FEBRABAN, the factor restarted at 1000 on 2025-02-22, so the same factor
// now maps to dates 9000 days apart. [Bancario.DueDateAt] resolves the
// ambiguity against a reference date; see its documentation for the window.
//
// # Hybrid boletos
//
// Many banks print a Pix QR code next to the barcode ("boleto híbrido" or
// "bolepix"). [ParseHybrid] keeps both payment instruments together without
// depending on a Pix implementation.
package boleto
