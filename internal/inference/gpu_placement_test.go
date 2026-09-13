package inference

import "testing"

func TestHasEnoughNvidiaVRAM(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
		want   bool
	}{
		{name: "single GPU", output: "7899\n", want: true},
		{name: "second GPU available", output: "128\n4096\n", want: true},
		{name: "all GPUs occupied", output: "512\n1000\n", want: false},
		{name: "malformed output", output: "not available\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := hasEnoughNvidiaVRAM([]byte(test.output)); got != test.want {
				t.Fatalf("hasEnoughNvidiaVRAM(%q) = %t, want %t", test.output, got, test.want)
			}
		})
	}
}
