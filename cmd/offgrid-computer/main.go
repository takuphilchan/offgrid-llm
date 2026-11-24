package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func main() {
	if err := run(); err != nil {
		if f := flag.Lookup("browser-storage"); f != nil && f.Value.String() == "true" {
			computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": 1, "id": "browser-storage", "error": "computer_storage_unavailable", "result": nil})
			fmt.Fprintln(os.Stderr, "OffGrid browser storage check failed")
			os.Exit(1)
		}
		protocol := 2
		if f := flag.Lookup("protocol"); f != nil && f.Value.String() == "3" {
			protocol = 3
		}
		computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": protocol, "id": "startup", "error": nativeErrorCode(err), "result": nil})
		fmt.Fprintln(os.Stderr, "OffGrid native worker stopped:", err)
		os.Exit(1)
	}
}
