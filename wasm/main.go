//go:build js && wasm

// Command wasm is the WebAssembly build of brpay used by the playground in
// /docs. It registers global JavaScript functions and then blocks:
//
//	brpayDecode(input: string): string            // JSON report, as "brpay decode --json"
//	brpayPixStatic(options: string): string       // JSON {payload} or {error}
//
// options is a JSON object with the fields key, name, city, amount, txid
// and description. Build with:
//
//	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o docs/brpay.wasm ./wasm
package main

import (
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/victorcondackdev/brpay/internal/report"
)

func main() {
	js.Global().Set("brpayDecode", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return marshal(report.Report{Kind: "unknown", Error: "brpayDecode expects one string argument"})
		}
		return marshal(report.Decode(args[0].String(), time.Now()))
	}))
	js.Global().Set("brpayPixStatic", js.FuncOf(func(_ js.Value, args []js.Value) any {
		var o report.StaticOptions
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return marshal(report.Static{Error: "brpayPixStatic expects a JSON string argument"})
		}
		if err := json.Unmarshal([]byte(args[0].String()), &o); err != nil {
			return marshal(report.Static{Error: "invalid options: " + err.Error()})
		}
		return marshal(report.BuildStatic(o))
	}))
	// Let the page know the functions are ready.
	if cb := js.Global().Get("brpayReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
	select {}
}

func marshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"internal: cannot encode result"}`
	}
	return string(b)
}
