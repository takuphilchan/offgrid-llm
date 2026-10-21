package cli

import "context"

func benchmarkCommand(ctx context.Context, args []string, env Environment) error {
	return runBenchmarkLegacy(ctx, args, env)
}

func benchmarkCompareCommand(ctx context.Context, args []string, env Environment) error {
	return runBenchmarkCompareLegacy(ctx, args, env)
}

func RunBenchmarkCommand(ctx context.Context, args []string, env Environment) error {
	return benchmarkCommand(ctx, args, env)
}
func RunBenchmarkCompareCommand(ctx context.Context, args []string, env Environment) error {
	return benchmarkCompareCommand(ctx, args, env)
}
