package emv_test

import (
	"fmt"

	"github.com/victorcondackdev/brpay/emv"
)

func Example() {
	fields := []emv.Field{
		{ID: "00", Value: "01"},
		{ID: "26", Children: []emv.Field{
			{ID: "00", Value: "br.gov.bcb.pix"},
			{ID: "01", Value: "123e4567-e12b-12d1-a456-426655440000"},
		}},
		{ID: "52", Value: "0000"},
		{ID: "53", Value: "986"},
		{ID: "58", Value: "BR"},
		{ID: "59", Value: "Fulano de Tal"},
		{ID: "60", Value: "BRASILIA"},
		{ID: "62", Children: []emv.Field{{ID: "05", Value: "***"}}},
	}
	body, err := emv.Encode(fields)
	if err != nil {
		panic(err)
	}
	payload := emv.AppendCRC(body)
	fmt.Println(payload)
	fmt.Println(emv.VerifyCRC(payload))
	// Output:
	// 00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D
	// <nil>
}

func ExampleParse() {
	fields, err := emv.Parse("00020126180014br.gov.bcb.pix5802BR63041234")
	if err != nil {
		panic(err)
	}
	for _, f := range fields {
		fmt.Printf("%s %q\n", f.ID, f.Value)
		for _, c := range f.Children {
			fmt.Printf("  %s.%s %q\n", f.ID, c.ID, c.Value)
		}
	}
	// Output:
	// 00 "01"
	// 26 "0014br.gov.bcb.pix"
	//   26.00 "br.gov.bcb.pix"
	// 58 "BR"
	// 63 "1234"
}

func ExampleParseError() {
	_, err := emv.Parse("000201593001234567")
	fmt.Println(err)
	// Output:
	// emv: byte 6, field 59: length 30 exceeds remaining 8
}

func ExampleCRC16() {
	fmt.Println(emv.CRC16("123456789"))
	// Output: 29B1
}
