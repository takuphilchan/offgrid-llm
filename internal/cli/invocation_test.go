package cli

import (
	"reflect"
	"testing"
)

func TestParseInvocationDoesNotMutateArgs(t *testing.T) {
	for _, input := range [][]string{
		{"offgrid", "--json", "model", "list", "--no-color"},
		{"offgrid", "model", "--no-color", "list", "--json"},
	} {
		original := append([]string(nil), input...)
		filtered, options := ParseInvocation(input)
		if !reflect.DeepEqual(input, original) || !reflect.DeepEqual(filtered, []string{"offgrid", "model", "list"}) || !options.JSON || !options.NoColor {
			t.Fatalf("input=%v filtered=%v options=%+v", input, filtered, options)
		}
		filtered[0] = "changed"
		if !reflect.DeepEqual(input, original) {
			t.Fatal("filtered result aliases original argument slice")
		}
	}
}
