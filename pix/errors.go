package pix

import "errors"

// Errors returned by this package. They are usually wrapped with details;
// test for them with errors.Is.
var (
	ErrInvalidKey          = errors.New("pix: invalid key")
	ErrInvalidAmount       = errors.New("pix: invalid amount")
	ErrInvalidMerchantName = errors.New("pix: invalid merchant name")
	ErrInvalidMerchantCity = errors.New("pix: invalid merchant city")
	ErrInvalidTxID         = errors.New("pix: invalid txid")
	ErrInvalidDescription  = errors.New("pix: invalid description")
	ErrInvalidPostalCode   = errors.New("pix: invalid postal code")
	ErrInvalidMCC          = errors.New("pix: invalid merchant category code")
	ErrInvalidURL          = errors.New("pix: invalid URL")
	ErrPayloadTooLong      = errors.New("pix: payload too long")
	ErrNotPix              = errors.New("pix: no br.gov.bcb.pix merchant account template")
	ErrKeyAndURL           = errors.New("pix: merchant account has both a key and a URL")
	ErrNoKeyOrURL          = errors.New("pix: merchant account has neither a key nor a URL")
	ErrMissingField        = errors.New("pix: missing required field")
	ErrInvalidEndToEndID   = errors.New("pix: invalid EndToEndId")
)
