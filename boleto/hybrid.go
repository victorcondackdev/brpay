package boleto

import (
	"fmt"
	"strings"
)

// pixPayloadPrefix is the EMV "Payload Format Indicator" (ID 00, length 02,
// value 01) that every Pix BR Code starts with.
const pixPayloadPrefix = "000201"

// Hybrid is a "boleto híbrido" (also marketed as "bolepix"): a boleto that
// also carries a Pix QR code, so the payer can choose either rail.
type Hybrid struct {
	// Boleto is the parsed and validated boleto.
	Boleto *Boleto
	// PixPayload is the raw Pix "copia e cola" payload, trimmed of
	// surrounding whitespace. Decode it with a Pix package.
	PixPayload string
}

// ParseHybrid parses the boleto in barcodeOrLine (any form accepted by
// [Parse]) and attaches the Pix payload printed next to it. The payload is
// only sanity-checked (it must start with the EMV payload format indicator
// "000201"); full BR Code validation, including the CRC, is left to a Pix
// decoder.
func ParseHybrid(barcodeOrLine, pixPayload string) (*Hybrid, error) {
	b, err := Parse(barcodeOrLine)
	if err != nil {
		return nil, err
	}
	p := strings.TrimSpace(pixPayload)
	if !strings.HasPrefix(p, pixPayloadPrefix) {
		return nil, fmt.Errorf("%w: missing %q prefix", ErrPixPayload, pixPayloadPrefix)
	}
	return &Hybrid{Boleto: b, PixPayload: p}, nil
}
