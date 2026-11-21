package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/victorcondackdev/brpay/internal/report"
)

// writeReport prints a report as aligned "label  value" lines, skipping
// empty values, followed by lint issues and boleto suggestions. The error
// itself goes to stderr.
func writeReport(w io.Writer, r report.Report) {
	var rows [][2]string
	add := func(label, value string) {
		if value != "" {
			rows = append(rows, [2]string{label, value})
		}
	}
	add("kind", r.Kind)
	if p := r.Pix; p != nil {
		add("initiation", p.Initiation)
		add("reusable", strconv.FormatBool(p.Reusable))
		if p.Key != "" {
			add("key", strings.TrimSpace(p.Key+" "+paren(p.KeyType)))
		}
		add("url", p.URL)
		add("merchant", p.MerchantName)
		add("city", p.MerchantCity)
		add("postal code", p.PostalCode)
		add("mcc", p.MCC)
		add("currency", p.Currency)
		if p.Amount == "" {
			add("amount", "open (payer chooses)")
		} else {
			add("amount", "R$ "+reais(p.AmountCents))
		}
		add("txid", p.TxID)
		add("description", p.Description)
		add("crc", p.CRC)
		ids := make([]string, 0, len(p.Templates))
		for id := range p.Templates {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			add("template "+id, p.Templates[id])
		}
	}
	if b := r.Boleto; b != nil {
		add("type", b.Type)
		add("formatted", b.Formatted)
		add("barcode", b.Barcode)
		add("digitable line", b.DigitableLine)
		if b.BankCode != "" {
			add("bank", strings.TrimSpace(b.BankCode+" "+b.BankName))
		}
		if b.SegmentName != "" {
			add("segment", fmt.Sprintf("%d %s", b.Segment, b.SegmentName))
		}
		if b.ValueIsReference {
			add("reference value", strconv.FormatInt(b.AmountCents, 10))
		} else {
			add("amount", "R$ "+reais(b.AmountCents))
		}
		add("due date", b.DueDate)
		if b.Factor > 0 {
			add("factor", strconv.Itoa(b.Factor))
		}
		add("company id", b.CompanyID)
		add("cnpj base", b.CNPJBase)
		add("free field", b.FreeField)
	}
	if d := r.Document; d != nil {
		add("type", d.Type)
		add("number", d.Number)
		add("formatted", d.Formatted)
		add("masked", d.Masked)
		add("alphanumeric", strconv.FormatBool(d.Alphanumeric))
	}
	if k := r.PixKey; k != nil {
		add("type", k.Type)
		add("key", k.Key)
	}
	if e := r.EndToEndID; e != nil {
		add("ispb", e.ISPB)
		add("time", e.Time)
		add("sequence", e.Sequence)
	}

	width := 0
	for _, row := range rows {
		width = max(width, len(row[0]))
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s  %s\n", width, row[0], row[1])
	}
	writeIssues(w, r.Issues, "issues:")
	if len(r.Suggestions) > 0 {
		fmt.Fprintln(w, "did you mean:")
		for _, s := range r.Suggestions {
			fmt.Fprintln(w, "  "+s.Formatted)
		}
	}
}

func writeIssues(w io.Writer, issues []report.Issue, header string) {
	if len(issues) == 0 {
		return
	}
	indent := ""
	if header != "" {
		fmt.Fprintln(w, header)
		indent = "  "
	}
	for _, is := range issues {
		if is.Field == "" {
			fmt.Fprintf(w, "%s%s: %s\n", indent, is.Severity, is.Message)
		} else {
			fmt.Fprintf(w, "%s%s [%s]: %s\n", indent, is.Severity, is.Field, is.Message)
		}
	}
}

func paren(s string) string {
	if s == "" {
		return ""
	}
	return "(" + s + ")"
}

// reais formats cents as Brazilian currency: 1234567 -> "12.345,67".
func reais(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	s := fmt.Sprintf("%s,%02d", b.String(), cents%100)
	if neg {
		s = "-" + s
	}
	return s
}
