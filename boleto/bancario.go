package boleto

import (
	"fmt"
	"strconv"
	"time"
)

const (
	bancarioFreeLen   = 25
	maxBancarioAmount = 9_999_999_999 // 10 digits, in cents
)

// Bancario holds the fields specific to a bank slip.
type Bancario struct {
	// BankCode is the 3-digit COMPE code of the issuing bank.
	BankCode string
	// BankName is the bank name from a small built-in table, or "".
	BankName string
	// Currency is the currency code; 9 means Brazilian real.
	Currency int
	// Factor is the due-date factor; 0 means no due date.
	Factor int
	// FreeField is the 25-digit bank-specific "campo livre".
	FreeField string
}

// bancarioFields lists, for the three linha digitável fields that carry a
// modulo-10 check digit, the start of the field and the index of its check
// digit.
var bancarioFields = [3][2]int{{0, 9}, {10, 20}, {21, 31}}

// bancarioRanges lists the [start, end) span of each of the five fields of a
// bancário linha digitável.
var bancarioRanges = [][2]int{{0, 10}, {10, 21}, {21, 32}, {32, 33}, {33, 47}}

// NewBancario builds a bank slip in Brazilian reais, computing every check
// digit. bank is the 3-digit COMPE code, a zero due means "no due date"
// (factor 0000), amountCents must fit ten digits and freeField must be the
// 25-digit bank-specific field.
func NewBancario(bank string, due time.Time, amountCents int64, freeField string) (*Boleto, error) {
	if len(bank) != 3 || !isDigits(bank) {
		return nil, fmt.Errorf("%w: %q", ErrBankCode, bank)
	}
	if bank[0] == arrecadacaoProduct {
		return nil, fmt.Errorf("%w: bank codes starting with 8 are reserved for arrecadação", ErrProductID)
	}
	if amountCents < 0 || amountCents > maxBancarioAmount {
		return nil, fmt.Errorf("%w: %d", ErrAmount, amountCents)
	}
	if len(freeField) != bancarioFreeLen || !isDigits(freeField) {
		return nil, fmt.Errorf("%w: want %d digits", ErrFreeField, bancarioFreeLen)
	}
	factor := 0
	if !due.IsZero() {
		f, err := FactorFor(due)
		if err != nil {
			return nil, err
		}
		factor = f
	}
	body := fmt.Sprintf("%s%d%04d%010d%s", bank, bancarioCurrencyBRL, factor, amountCents, freeField)
	code := body[:4] + string(mod11Bancario(body)) + body[4:]
	return buildBancario(code), nil
}

// DueDateAt resolves the due date relative to ref; see [DueDateAt] for the
// rules. It reports false when the boleto has no due date (factor 0000).
func (b Bancario) DueDateAt(ref time.Time) (time.Time, bool) {
	return DueDateAt(b.Factor, ref)
}

// DueDate is [Bancario.DueDateAt] with the current time as reference.
func (b Bancario) DueDate() (time.Time, bool) {
	return b.DueDateAt(time.Now())
}

func parseBancarioBarcode(code string) (*Boleto, error) {
	if code[0] == arrecadacaoProduct {
		return nil, fmt.Errorf("%w: bancário codes cannot start with 8", ErrProductID)
	}
	if want := mod11Bancario(code[:4] + code[5:]); code[4] != want {
		return nil, &CheckDigitError{Field: FieldGeneral, Expected: want, Got: code[4]}
	}
	return buildBancario(code), nil
}

func parseBancarioLine(line string) (*Boleto, error) {
	if line[0] == arrecadacaoProduct {
		return nil, fmt.Errorf("%w: bancário codes cannot start with 8", ErrProductID)
	}
	for i, f := range bancarioFields {
		if want := mod10(line[f[0]:f[1]]); line[f[1]] != want {
			return nil, &CheckDigitError{Field: i + 1, Expected: want, Got: line[f[1]]}
		}
	}
	code := line[0:4] + line[32:33] + line[33:47] + line[4:9] + line[10:20] + line[21:31]
	return parseBancarioBarcode(code)
}

// buildBancario assembles a Boleto from a barcode whose DAC is known to be
// valid.
func buildBancario(code string) *Boleto {
	f1 := code[0:4] + code[19:24]
	f2 := code[24:34]
	f3 := code[34:44]
	line := make([]byte, 0, BancarioLineLen)
	line = append(line, f1...)
	line = append(line, mod10(f1))
	line = append(line, f2...)
	line = append(line, mod10(f2))
	line = append(line, f3...)
	line = append(line, mod10(f3))
	line = append(line, code[4:19]...)

	factor, _ := strconv.Atoi(code[5:9])
	amount, _ := strconv.ParseInt(code[9:19], 10, 64)
	return &Boleto{
		Type:          TypeBancario,
		Barcode:       code,
		DigitableLine: string(line),
		AmountCents:   amount,
		Bancario: &Bancario{
			BankCode:  code[0:3],
			BankName:  BankName(code[0:3]),
			Currency:  int(code[3] - '0'),
			Factor:    factor,
			FreeField: code[19:44],
		},
	}
}
