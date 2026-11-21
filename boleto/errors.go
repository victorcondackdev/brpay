package boleto

import (
	"errors"
	"fmt"
)

// Sentinel errors returned (possibly wrapped) by this package. Use
// [errors.Is] to test for them.
var (
	// ErrLength reports an input whose digit count is not 44, 47 or 48.
	ErrLength = errors.New("boleto: invalid length")
	// ErrCharacters reports a character other than digits and the
	// accepted separators (space, tab, newline, ".", "-").
	ErrCharacters = errors.New("boleto: invalid character")
	// ErrCheckDigit reports a check digit mismatch. The concrete error is a
	// [*CheckDigitError] carrying the details.
	ErrCheckDigit = errors.New("boleto: check digit mismatch")
	// ErrProductID reports an arrecadação code that does not start with 8,
	// or a bancário code that does (bank codes starting with 8 are reserved
	// for arrecadação).
	ErrProductID = errors.New("boleto: invalid product identifier")
	// ErrValueType reports an arrecadação value-type identifier (third
	// digit) other than 6, 7, 8 or 9.
	ErrValueType = errors.New("boleto: invalid value type identifier")
	// ErrBankCode reports a bank code that is not three digits.
	ErrBankCode = errors.New("boleto: invalid bank code")
	// ErrAmount reports an amount that is negative or does not fit the
	// barcode's amount field.
	ErrAmount = errors.New("boleto: amount out of range")
	// ErrFreeField reports a free field of the wrong size or with non-digits.
	ErrFreeField = errors.New("boleto: invalid free field")
	// ErrSegment reports an unknown arrecadação segment.
	ErrSegment = errors.New("boleto: invalid segment")
	// ErrDueDate reports a due date that cannot be encoded as a factor.
	ErrDueDate = errors.New("boleto: due date out of range")
	// ErrPixPayload reports a Pix payload that does not look like a BR Code.
	ErrPixPayload = errors.New("boleto: invalid Pix payload")
)

// FieldGeneral is the [CheckDigitError.Field] value used for the general
// check digit (DAC) of the whole code. In a bancário linha digitável it is
// printed as field 4; in an arrecadação code it is the fourth digit.
const FieldGeneral = 0

// CheckDigitError describes a check digit that does not match its data, so
// callers can point users at the exact block they mistyped.
//
// Field is the 1-based field (bancário) or block (arrecadação) of the linha
// digitável whose own check digit failed, or [FieldGeneral] for the general
// check digit. Expected and Got are ASCII digits ('0'–'9').
//
// CheckDigitError matches [ErrCheckDigit] with [errors.Is].
type CheckDigitError struct {
	Field    int
	Expected byte
	Got      byte
}

// Error implements the error interface.
func (e *CheckDigitError) Error() string {
	if e.Field == FieldGeneral {
		return fmt.Sprintf("boleto: general check digit mismatch: expected %c, got %c", e.Expected, e.Got)
	}
	return fmt.Sprintf("boleto: check digit mismatch in field %d: expected %c, got %c", e.Field, e.Expected, e.Got)
}

// Is reports whether target is [ErrCheckDigit].
func (e *CheckDigitError) Is(target error) bool { return target == ErrCheckDigit }
