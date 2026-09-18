package main

import (
	"io"
	"testing"
)

func TestModelSearchOptions(t *testing.T) {
	filter, ram, files, err := parseModelSearchArgs([]string{"large", "model", "--files", "--limit", "5", "--quant", "Q8_0"})
	if err != nil || filter.Query != "large model" || ram != 0 || !files || filter.Limit != 5 || filter.Quantization != "Q8_0" {
		t.Fatalf("%+v RAM=%d files=%v err=%v", filter, ram, files, err)
	}
	for _, args := range [][]string{{"--wat"}, {"--ram"}, {"--ram", "bad"}, {"--limit", "0"}, {"--limit", "51"}, {"--sort", "bad"}, {"--author", "--files"}} {
		_, _, _, err := parseModelSearchArgs(args)
		if code := reportCommandError(err, true, io.Discard, io.Discard); code != 2 {
			t.Errorf("%v: exit %d, err %v", args, code, err)
		}
	}
}
