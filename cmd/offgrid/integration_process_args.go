package main

import (
	"fmt"
	"strings"
)

// External integration commands are explicit user-facing pass-throughs. Keep
// their existing verbs, but bound the argv handed to an installed executable.
// No shell interpretation is used by the launchers.
func validateIntegrationArgs(provider string, args []string) error {
	if len(args) > 128 {
		return fmt.Errorf("%s invocation has too many arguments", provider)
	}
	total := 0
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) || len(arg) > 64<<10 {
			return fmt.Errorf("%s invocation contains an invalid argument", provider)
		}
		total += len(arg)
		if total > 256<<10 {
			return fmt.Errorf("%s invocation exceeds the argument budget", provider)
		}
	}
	return nil
}
