package boleto

// banks maps COMPE codes of major Brazilian banks to their names.
var banks = map[string]string{
	"001": "Banco do Brasil",
	"033": "Santander",
	"041": "Banrisul",
	"070": "BRB",
	"077": "Inter",
	"104": "Caixa Econômica Federal",
	"212": "Banco Original",
	"237": "Bradesco",
	"260": "Nu Pagamentos",
	"336": "C6 Bank",
	"341": "Itaú Unibanco",
	"422": "Safra",
	"745": "Citibank",
	"748": "Sicredi",
	"756": "Sicoob",
}

// BankName returns the name of the bank with the given 3-digit COMPE code
// from a small built-in table of major banks, or "" when unknown.
func BankName(code string) string { return banks[code] }
