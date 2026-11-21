# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-21

First public release.

### Added

- `brpay`: `Detect` and `Decode`, a single entry point that recognizes Pix
  copia e cola payloads, boleto barcodes and linhas digitáveis, CPF/CNPJ,
  Pix keys and EndToEndIds, returning lint issues and typo suggestions
  alongside the decoded value.
- `pix`: BR Code `Decode`, `NewStatic`, `NewDynamic`; `Lint` with
  field-level diagnostics; Pix key detection and normalization for all five
  key types; EndToEndId parsing and generation.
- `boleto`: `Parse` and `Validate` for 44-, 47- and 48-digit codes, bancário
  and arrecadação; barcode and linha digitável conversions; due-date factor
  with the 2025-02-22 rollover; `NewBancario`, `NewArrecadacao`;
  `ParseHybrid` for boletos with a Pix QR code; `SuggestFix` for
  single-digit typos; `CheckDigitError` naming the faulty field.
- `document`: CPF and CNPJ validation, formatting and masking, including the
  alphanumeric CNPJ.
- `emv`: EMV MPM TLV parsing, encoding and validation, CRC16/CCITT-FALSE,
  positional `ParseError`.
- `cmd/brpay`: CLI with `decode`, `lint`, `pix static`, `boleto fix` and
  `version`; human-readable or `--json` output.
- WebAssembly playground served from `docs/` on GitHub Pages.

[Unreleased]: https://github.com/victorcondackdev/brpay/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/victorcondackdev/brpay/releases/tag/v0.1.0
