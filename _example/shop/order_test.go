package shop

import "testing"

func TestParseQuantity(t *testing.T) {
	if n, err := ParseQuantity("3"); err != nil || n != 3 {
		t.Fatalf("ParseQuantity(\"3\") = %d, %v", n, err)
	}
}
