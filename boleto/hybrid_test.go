package boleto

import (
	"errors"
	"testing"
)

const samplePix = "00020101021226580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

func TestParseHybrid(t *testing.T) {
	h, err := ParseHybrid(handLine, "  "+samplePix+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.PixPayload != samplePix || h.Boleto.Barcode != handBarcode {
		t.Errorf("unexpected %+v", h)
	}
	if _, err := ParseHybrid(handLine, "hello"); !errors.Is(err, ErrPixPayload) {
		t.Errorf("bad pix: %v", err)
	}
	if _, err := ParseHybrid("123", samplePix); !errors.Is(err, ErrLength) {
		t.Errorf("bad boleto: %v", err)
	}
}
