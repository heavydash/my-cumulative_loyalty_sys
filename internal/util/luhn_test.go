package util

import "testing"

func TestIsLuhnValid(t *testing.T) {
	tests := []struct {
		name string
		num  string
		want bool
	}{
		{"valid classic", "79927398713", true},
		{"valid another", "45393122383", false},
		{"invalid simple", "123456789032", false},
		{"invalid all same", "11111111111", false},
		{"valid Amex", "378282246310005", true},
		{"valid long", "49927398716", true},
		{"empty string", "", false},
		{"non digits", "abc123", false},
		{"zeroes", "00000000000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLuhnValid(tt.num)
			if got != tt.want {
				t.Errorf("IsLuhnValid() = %v, want %v", got, tt.want)
			}
		})
	}

}
