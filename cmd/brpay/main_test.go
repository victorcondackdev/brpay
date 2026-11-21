package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const (
	bbLine     = "00190.00009 00000.000000 00000.000075 9 10000000010000"
	bbLineTypo = "00190.00009 00000.500000 00000.000075 9 10000000010000"
	staticPix  = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"
)

func init() {
	now = func() time.Time { return time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC) }
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		code       int
		stdout     string // exact, unless prefixed with "~" (substring)
		stderr     string // substring; "" means stderr must be empty
		jsonOutput bool
	}{
		{name: "no args", code: 2, stderr: "Usage:"},
		{name: "help", args: []string{"help"}, stdout: "~brpay decode [--json]"},
		{name: "unknown command", args: []string{"frobnicate"}, code: 2, stderr: `unknown command "frobnicate"`},
		{name: "bad flag", args: []string{"decode", "--nope", "x"}, code: 2, stderr: "flag provided but not defined"},
		{name: "decode help flag", args: []string{"decode", "-h"}, stdout: "~Usage:", stderr: "usage: brpay decode"},
		{
			name: "decode document",
			args: []string{"decode", "529.982.247-25"},
			stdout: "kind          document\n" +
				"type          cpf\n" +
				"number        52998224725\n" +
				"formatted     529.982.247-25\n" +
				"masked        ***.982.247-**\n" +
				"alphanumeric  false\n",
		},
		{
			name: "decode unquoted linha digitável",
			args: append([]string{"decode"}, strings.Fields(bbLine)...),
			stdout: "kind            boleto\n" +
				"type            bancario\n" +
				"formatted       " + bbLine + "\n" +
				"barcode         00199100000000100000000000000000000000000007\n" +
				"digitable line  00190000090000000000000000000075910000000010000\n" +
				"bank            001 Banco do Brasil\n" +
				"amount          R$ 100,00\n" +
				"due date        2025-02-22\n" +
				"factor          1000\n" +
				"free field      0000000000000000000000007\n",
		},
		{
			name: "decode pix from stdin",
			args: []string{"decode"}, stdin: staticPix + "\n",
			stdout: "kind        pix\n" +
				"initiation  static\n" +
				"reusable    true\n" +
				"key         123e4567-e12b-12d1-a456-426655440000 (evp)\n" +
				"merchant    Fulano de Tal\n" +
				"city        BRASILIA\n" +
				"mcc         0000\n" +
				"currency    986\n" +
				"amount      open (payer chooses)\n" +
				"crc         1D3D\n",
		},
		{name: "decode dash reads stdin", args: []string{"decode", "-"}, stdin: "+5521987654321", stdout: "~key   +5521987654321\n"},
		{name: "decode flag after input", args: []string{"decode", "fulano@example.com", "--json"}, jsonOutput: true,
			stdout: `{"kind":"pix_key","valid":true,"pix_key":{"type":"email","key":"fulano@example.com"}}`},
		{name: "decode double dash", args: []string{"decode", "--json", "--", "-x"}, code: 1, jsonOutput: true,
			stdout: `{"kind":"unknown","valid":false,"error":"brpay: unrecognized input"}`},
		{name: "decode unrecognized", args: []string{"decode", "hello"}, code: 1, stdout: "kind  unknown\n", stderr: "brpay: brpay: unrecognized input"},
		{
			name: "decode broken pix shows issues", args: []string{"decode", strings.Replace(staticPix, "1D3D", "1D3E", 1)}, code: 1,
			stdout: "kind  pix\nissues:\n  error [63]: CRC is \"1D3E\", expected 1D3D (was the payload edited after the CRC was computed?)\n",
			stderr: "CRC mismatch",
		},
		{
			name: "decode typo suggests", args: []string{"decode", bbLineTypo}, code: 1,
			stdout: "kind  boleto\ndid you mean:\n  00190.00009 00009.500000 00000.000075 9 10000000010000\n  " + bbLine + "\n",
			stderr: "check digit mismatch in field 2",
		},
		{name: "lint ok", args: []string{"lint", staticPix}, stdout: "ok: no issues found\n"},
		{name: "lint warning only", args: []string{"lint"}, stdin: " " + staticPix, stdout: "~warning: payload has surrounding whitespace"},
		{name: "lint error", args: []string{"lint", "000201"}, code: 1, stdout: "~error", stderr: "issue(s)"},
		{name: "lint json", args: []string{"lint", "--json", staticPix}, jsonOutput: true, stdout: `{"valid":true,"issues":[]}`},
		{
			name:   "pix static",
			args:   []string{"pix", "static", "--key", "+55 (21) 98765-4321", "--name", "Padaria Pão Quente", "--city", "Niterói", "--amount", "12.50", "--txid", "PEDIDO42"},
			stdout: "00020126360014br.gov.bcb.pix0114+5521987654321520400005303986540512.505802BR5918Padaria Pao Quente6007Niteroi62120508PEDIDO4263048559\n",
		},
		{name: "pix static json", args: []string{"pix", "static", "--json", "--key", "a@b.co", "--name", "A", "--city", "B"}, jsonOutput: true,
			stdout: `{"payload":"00020126280014br.gov.bcb.pix0106a@b.co5204000053039865802BR5901A6001B62070503***6304FAC1"}`},
		{name: "pix static missing flags", args: []string{"pix", "static", "--key", "a@b.co"}, code: 2, stderr: "are required"},
		{name: "pix static bad amount", args: []string{"pix", "static", "--key", "a@b.co", "--name", "A", "--city", "B", "--amount", "1.000,00"}, code: 1, stderr: "invalid amount"},
		{name: "pix static bad key", args: []string{"pix", "static", "--key", "nope", "--name", "A", "--city", "B"}, code: 1, stderr: "pix: invalid key"},
		{name: "pix static stray arg", args: []string{"pix", "static", "extra"}, code: 2, stderr: "unexpected argument"},
		{name: "pix without subcommand", args: []string{"pix"}, code: 2, stderr: "usage: brpay pix static"},
		{name: "boleto fix valid", args: []string{"boleto", "fix", bbLine}, stdout: "ok: valid bancario boleto\n" + bbLine + "\n"},
		{name: "boleto fix typo", args: []string{"boleto", "fix"}, stdin: bbLineTypo + "\r\n", code: 1,
			stdout: "did you mean:\n  00190.00009 00009.500000 00000.000075 9 10000000010000\n  " + bbLine + "\n", stderr: "field 2"},
		{name: "boleto fix not a boleto", args: []string{"boleto", "fix", "529.982.247-25"}, code: 1, stderr: "not a boleto"},
		{name: "boleto without subcommand", args: []string{"boleto", "parse"}, code: 2, stderr: "usage: brpay boleto fix"},
		{name: "version", args: []string{"version"}, stdout: "~brpay dev (go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if code != tt.code {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.code, &stdout, &stderr)
			}
			got := stdout.String()
			if tt.jsonOutput {
				var buf bytes.Buffer
				if err := json.Compact(&buf, stdout.Bytes()); err != nil {
					t.Fatalf("invalid JSON %q: %v", got, err)
				}
				got = buf.String()
			}
			if want, ok := strings.CutPrefix(tt.stdout, "~"); ok {
				if !strings.Contains(got, want) {
					t.Errorf("stdout = %q, want it to contain %q", got, want)
				}
			} else if got != tt.stdout {
				t.Errorf("stdout:\n%s\nwant:\n%s", got, tt.stdout)
			}
			if tt.stderr == "" && stderr.Len() > 0 {
				t.Errorf("unexpected stderr: %q", &stderr)
			}
			if !strings.Contains(stderr.String(), tt.stderr) {
				t.Errorf("stderr = %q, want it to contain %q", &stderr, tt.stderr)
			}
		})
	}
}

func TestVersionFromLdflags(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "v0.1.0"
	var out bytes.Buffer
	if code := run([]string{"version"}, nil, &out, &out); code != 0 || !strings.HasPrefix(out.String(), "brpay v0.1.0 (go") {
		t.Fatalf("%d %q", code, &out)
	}
}

func TestStdinLimit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"decode"}, strings.NewReader(strings.Repeat("0", maxInput+1)), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "exceeds") {
		t.Fatalf("%d %q", code, &stderr)
	}
}

func TestReais(t *testing.T) {
	for cents, want := range map[int64]string{0: "0,00", 5: "0,05", 100: "1,00", 123456: "1.234,56", 99999999999999: "999.999.999.999,99", -150: "-1,50"} {
		if got := reais(cents); got != want {
			t.Errorf("reais(%d) = %q, want %q", cents, got, want)
		}
	}
}
