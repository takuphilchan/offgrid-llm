package cli

// ParseInvocation removes global flags from a copy of the argument slice.
// The caller decides how options affect a particular command; no process
// argument or environment state is changed here.
func ParseInvocation(args []string) ([]string, InvocationOptions) {
	filtered := make([]string, 0, len(args))
	var options InvocationOptions
	for _, arg := range args {
		switch arg {
		case "--json":
			options.JSON = true
		case "--no-color":
			options.NoColor = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, options
}
