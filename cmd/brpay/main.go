// Command brpay decodes, lints and builds Brazilian payment codes: Pix
// "copia e cola" payloads, boletos, CPF/CNPJ, Pix keys and EndToEndIds.
//
// Run "brpay help" for usage.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/victorcondackdev/brpay/internal/report"
	"github.com/victorcondackdev/brpay/pix"
)

// version is set at build time with -ldflags "-X main.version=v0.1.0".
var version = "dev"

// now is the reference time for boleto due dates; replaced in tests.
var now = time.Now

// Exit codes.
const (
	exitOK      = 0
	exitInvalid = 1
	exitUsage   = 2
)

// maxInput bounds what is read from stdin; real payloads are far smaller.
const maxInput = 64 << 10

const usage = `brpay decodes, lints and builds Brazilian payment codes.

Usage:
  brpay decode [--json] [input | -]
        Detect and decode a Pix copia e cola, boleto barcode or linha
        digitável, CPF/CNPJ, Pix key or EndToEndId.
  brpay lint [--json] [payload | -]
        Report every problem in a Pix copia e cola payload.
  brpay pix static [--json] --key KEY --name NAME --city CITY
                   [--amount 10.50] [--txid ID] [--desc TEXT]
        Build a static Pix copia e cola payload.
  brpay boleto fix [--json] [linha | -]
        Suggest corrections for a linha digitável with one mistyped digit.
  brpay version
        Print the version.

Input is read from standard input when it is omitted or "-". Several
arguments are joined with spaces, so a linha digitável can be pasted
unquoted.

Exit status: 0 success, 1 invalid input, 2 usage error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the whole program, minus process-level concerns, for testing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	c := &cli{stdin: stdin, stdout: stdout, stderr: stderr}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "decode":
		return c.decode(rest)
	case "lint":
		return c.lint(rest)
	case "pix":
		if len(rest) > 0 && rest[0] == "static" {
			return c.pixStatic(rest[1:])
		}
		return c.usageError("usage: brpay pix static --key KEY --name NAME --city CITY [flags]")
	case "boleto":
		if len(rest) > 0 && rest[0] == "fix" {
			return c.boletoFix(rest[1:])
		}
		return c.usageError("usage: brpay boleto fix [--json] [linha | -]")
	case "version", "--version", "-version":
		fmt.Fprintf(stdout, "brpay %s (%s)\n", resolveVersion(), runtime.Version())
		return exitOK
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		return c.usageError(fmt.Sprintf("unknown command %q; run \"brpay help\"", cmd))
	}
}

type cli struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	json           bool
}

func (c *cli) usageError(msg string) int {
	fmt.Fprintln(c.stderr, "brpay:", msg)
	return exitUsage
}

// fail reports invalid input. In JSON mode the error is already part of
// the document on stdout, so stderr stays quiet.
func (c *cli) fail(err error) int {
	if !c.json {
		fmt.Fprintln(c.stderr, "brpay:", err)
	}
	return exitInvalid
}

// flags returns a flag set with the shared --json flag.
func (c *cli) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	fs.BoolVar(&c.json, "json", false, "print JSON")
	fs.Usage = func() { fmt.Fprintf(c.stderr, "usage: brpay %s; run \"brpay help\"\n", name) }
	return fs
}

// parse parses flags interspersed with positional arguments, which the
// flag package alone does not support, and returns the positionals.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if n := len(args) - len(rest); n > 0 && args[n-1] == "--" {
			return append(pos, rest...), nil
		}
		if len(rest) == 0 {
			return pos, nil
		}
		pos, args = append(pos, rest[0]), rest[1:]
	}
}

// input returns the positional arguments joined by spaces, or standard
// input (minus one trailing line break) when there are none or just "-".
func (c *cli) input(pos []string) (string, error) {
	if len(pos) > 0 && !(len(pos) == 1 && pos[0] == "-") {
		return strings.Join(pos, " "), nil
	}
	b, err := io.ReadAll(io.LimitReader(c.stdin, maxInput+1))
	if err != nil {
		return "", fmt.Errorf("reading standard input: %w", err)
	}
	if len(b) > maxInput {
		return "", fmt.Errorf("standard input exceeds %d bytes", maxInput)
	}
	s := strings.TrimSuffix(string(b), "\n")
	return strings.TrimSuffix(s, "\r"), nil
}

func (c *cli) readInput(name string, args []string) (string, int) {
	fs := c.flags(name)
	pos, err := parse(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(c.stdout, usage)
		return "", exitOK
	}
	if err != nil {
		return "", exitUsage
	}
	in, err := c.input(pos)
	if err != nil {
		return "", c.fail(err)
	}
	return in, -1
}

func (c *cli) decode(args []string) int {
	in, code := c.readInput("decode", args)
	if code >= 0 {
		return code
	}
	rep := report.Decode(in, now())
	if c.json {
		c.writeJSON(rep)
	} else {
		writeReport(c.stdout, rep)
	}
	if !rep.Valid {
		return c.fail(errors.New(rep.Error))
	}
	return exitOK
}

func (c *cli) lint(args []string) int {
	in, code := c.readInput("lint", args)
	if code >= 0 {
		return code
	}
	var out struct {
		Valid  bool           `json:"valid"`
		Issues []report.Issue `json:"issues"`
	}
	out.Valid, out.Issues = true, []report.Issue{}
	for _, is := range pix.Lint(in) {
		out.Valid = out.Valid && is.Severity != pix.SeverityError
		out.Issues = append(out.Issues, report.Issue{Severity: is.Severity.String(), Field: is.Field, Message: is.Message})
	}
	if c.json {
		c.writeJSON(out)
	} else {
		if len(out.Issues) == 0 {
			fmt.Fprintln(c.stdout, "ok: no issues found")
		}
		writeIssues(c.stdout, out.Issues, "")
	}
	if !out.Valid {
		return c.fail(fmt.Errorf("payload has %d issue(s)", len(out.Issues)))
	}
	return exitOK
}

func (c *cli) pixStatic(args []string) int {
	var o report.StaticOptions
	fs := c.flags("pix static")
	fs.StringVar(&o.Key, "key", "", "Pix key: CPF, CNPJ, e-mail, +55 phone or EVP (required)")
	fs.StringVar(&o.Name, "name", "", "receiver name, up to 25 characters (required)")
	fs.StringVar(&o.City, "city", "", "receiver city, up to 15 characters (required)")
	fs.StringVar(&o.Amount, "amount", "", "amount in reais, e.g. 10.50; omit to let the payer choose")
	fs.StringVar(&o.TxID, "txid", "", "transaction id, up to 25 alphanumeric characters")
	fs.StringVar(&o.Description, "desc", "", "free-text description shown to the payer")
	pos, err := parse(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		return exitUsage
	}
	if len(pos) > 0 {
		return c.usageError(fmt.Sprintf("pix static: unexpected argument %q", pos[0]))
	}
	if o.Key == "" || o.Name == "" || o.City == "" {
		return c.usageError("pix static: --key, --name and --city are required")
	}
	res := report.BuildStatic(o)
	if c.json {
		c.writeJSON(res)
	} else if res.Payload != "" {
		fmt.Fprintln(c.stdout, res.Payload)
	}
	if res.Error != "" {
		return c.fail(errors.New(res.Error))
	}
	return exitOK
}

func (c *cli) boletoFix(args []string) int {
	in, code := c.readInput("boleto fix", args)
	if code >= 0 {
		return code
	}
	rep := report.Decode(in, now())
	if rep.Kind != "boleto" {
		rep = report.Report{Kind: rep.Kind, Error: "not a boleto barcode or linha digitável"}
	}
	if c.json {
		c.writeJSON(rep)
	} else if rep.Valid {
		fmt.Fprintln(c.stdout, "ok: valid", rep.Boleto.Type, "boleto")
		fmt.Fprintln(c.stdout, rep.Boleto.Formatted)
	} else if len(rep.Suggestions) > 0 {
		fmt.Fprintln(c.stdout, "did you mean:")
		for _, s := range rep.Suggestions {
			fmt.Fprintln(c.stdout, "  "+s.Formatted)
		}
	}
	if !rep.Valid {
		return c.fail(errors.New(rep.Error))
	}
	return exitOK
}

func (c *cli) writeJSON(v any) {
	enc := json.NewEncoder(c.stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// resolveVersion prefers the -ldflags version, then the module version
// recorded by "go install module@version".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
