package common

import (
	"testing"
)

func TestShortenName(t *testing.T) {
	inputs := []string{
		"John Doe",
		"John Doe Jr.",
		"0xc17227d7ab78ae1711f3297179a14eb05ec504b515a0b176bdf18d21c7bf5512",
	}

	expected := []string{
		"Jo__oe",
		"Jo__r.",
		"0x__12",
	}

	length := 2

	for i, input := range inputs {
		output := ShortenName(input, 2)
		expectedOutput := expected[i]
		if output != expectedOutput {
			t.Errorf("ShortenName(%q, %d) = %q, want %q", input, length, output, expectedOutput)
		}
	}
}
