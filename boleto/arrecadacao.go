package boleto

import (
	"fmt"
	"strconv"
)

const (
	arrecadacaoRestLen   = 29             // company/agency id + free field
	maxArrecadacaoAmount = 99_999_999_999 // 11 digits
)

// Segment is the arrecadação segment identifier (second barcode digit).
type Segment int

// Arrecadação segments defined by FEBRABAN.
const (
	SegmentPrefeituras    Segment = 1 // city halls
	SegmentSaneamento     Segment = 2 // water and sanitation
	SegmentEnergiaGas     Segment = 3 // electricity and gas
	SegmentTelecom        Segment = 4 // telecommunications
	SegmentGoverno        Segment = 5 // government agencies
	SegmentCNPJ           Segment = 6 // carnês and others identified by CNPJ
	SegmentMultasTransito Segment = 7 // traffic fines
	SegmentBanco          Segment = 9 // reserved for the bank
)

// String returns the FEBRABAN (Portuguese) name of the segment, or "" when
// the segment is not defined.
func (s Segment) String() string {
	switch s {
	case SegmentPrefeituras:
		return "Prefeituras"
	case SegmentSaneamento:
		return "Saneamento"
	case SegmentEnergiaGas:
		return "Energia elétrica e gás"
	case SegmentTelecom:
		return "Telecomunicações"
	case SegmentGoverno:
		return "Órgãos governamentais"
	case SegmentCNPJ:
		return "Carnês e demais empresas identificadas por CNPJ"
	case SegmentMultasTransito:
		return "Multas de trânsito"
	case SegmentBanco:
		return "Uso exclusivo do banco"
	default:
		return ""
	}
}

// Arrecadacao holds the fields specific to utility bills, taxes and fines.
type Arrecadacao struct {
	// Segment is the segment identifier.
	Segment Segment
	// ValueType is the value-type identifier (6, 7, 8 or 9).
	ValueType int
	// ValueIsReference is true for value types 7 and 9, where the amount
	// field is a reference (e.g. a quantity or index) rather than money.
	ValueIsReference bool
	// Modulus is 10 (value types 6, 7) or 11 (value types 8, 9).
	Modulus int
	// CompanyID is the 4-digit company/agency code; empty for SegmentCNPJ.
	CompanyID string
	// CNPJBase is the 8-digit CNPJ base; only set for SegmentCNPJ.
	CNPJBase string
	// FreeField is the remainder of the barcode after the identification.
	FreeField string
}

// arrecadacaoRanges lists the [start, end) span of each block of an
// arrecadação linha digitável.
var arrecadacaoRanges = [][2]int{{0, 12}, {12, 24}, {24, 36}, {36, 48}}

// NewArrecadacao builds an arrecadação code, computing every check digit.
// valueType selects the modulus and meaning of the value (6 or 7: modulo
// 10; 8 or 9: modulo 11; 7 and 9 mean "reference"). rest is the 29-digit
// tail: the company/agency identification followed by the free field.
func NewArrecadacao(segment Segment, valueType int, value int64, rest string) (*Boleto, error) {
	if segment.String() == "" {
		return nil, fmt.Errorf("%w: %d", ErrSegment, int(segment))
	}
	if valueType < 6 || valueType > 9 {
		return nil, fmt.Errorf("%w: %d", ErrValueType, valueType)
	}
	if value < 0 || value > maxArrecadacaoAmount {
		return nil, fmt.Errorf("%w: %d", ErrAmount, value)
	}
	if len(rest) != arrecadacaoRestLen || !isDigits(rest) {
		return nil, fmt.Errorf("%w: want %d digits", ErrFreeField, arrecadacaoRestLen)
	}
	body := fmt.Sprintf("8%d%d%011d%s", int(segment), valueType, value, rest)
	dv, _ := checkFunc(body[2])
	code := body[:3] + string(dv(body)) + body[3:]
	return buildArrecadacao(code, dv), nil
}

// checkFunc returns the check digit function selected by the value-type
// identifier.
func checkFunc(valueType byte) (func(string) byte, error) {
	switch valueType {
	case '6', '7':
		return mod10, nil
	case '8', '9':
		return mod11Arrecadacao, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrValueType, valueType)
	}
}

func parseArrecadacaoBarcode(code string) (*Boleto, error) {
	if code[0] != arrecadacaoProduct {
		return nil, fmt.Errorf("%w: arrecadação codes start with 8", ErrProductID)
	}
	dv, err := checkFunc(code[2])
	if err != nil {
		return nil, err
	}
	if want := dv(code[:3] + code[4:]); code[3] != want {
		return nil, &CheckDigitError{Field: FieldGeneral, Expected: want, Got: code[3]}
	}
	return buildArrecadacao(code, dv), nil
}

func parseArrecadacaoLine(line string) (*Boleto, error) {
	if line[0] != arrecadacaoProduct {
		return nil, fmt.Errorf("%w: arrecadação codes start with 8", ErrProductID)
	}
	dv, err := checkFunc(line[2])
	if err != nil {
		return nil, err
	}
	code := make([]byte, 0, BarcodeLen)
	for i, r := range arrecadacaoRanges {
		block, got := line[r[0]:r[1]-1], line[r[1]-1]
		if want := dv(block); got != want {
			return nil, &CheckDigitError{Field: i + 1, Expected: want, Got: got}
		}
		code = append(code, block...)
	}
	return parseArrecadacaoBarcode(string(code))
}

// buildArrecadacao assembles a Boleto from a barcode whose general check
// digit is known to be valid.
func buildArrecadacao(code string, dv func(string) byte) *Boleto {
	line := make([]byte, 0, ArrecadacaoLineLen)
	for i := 0; i < BarcodeLen; i += 11 {
		block := code[i : i+11]
		line = append(line, block...)
		line = append(line, dv(block))
	}
	vt := int(code[2] - '0')
	a := &Arrecadacao{
		Segment:          Segment(code[1] - '0'),
		ValueType:        vt,
		ValueIsReference: vt == 7 || vt == 9,
		Modulus:          10,
	}
	if vt >= 8 {
		a.Modulus = 11
	}
	if a.Segment == SegmentCNPJ {
		a.CNPJBase, a.FreeField = code[15:23], code[23:44]
	} else {
		a.CompanyID, a.FreeField = code[15:19], code[19:44]
	}
	value, _ := strconv.ParseInt(code[4:15], 10, 64)
	return &Boleto{
		Type:          TypeArrecadacao,
		Barcode:       code,
		DigitableLine: string(line),
		AmountCents:   value,
		Arrecadacao:   a,
	}
}
