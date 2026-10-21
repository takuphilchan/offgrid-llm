package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// reviewedLlamaServerArgs preserves the existing runtime flags while keeping
// model paths as argv data, never shell syntax.
func reviewedLlamaServerArgs(modelPath, port string, threads, contextSize, gpuLayers, batchSize int, lowRAM bool) ([]string, error) {
	portNumber, err := strconv.Atoi(port)
	if strings.TrimSpace(modelPath) == "" || strings.ContainsRune(modelPath, 0) || err != nil || portNumber < 1 || portNumber > 65535 || threads < 1 || threads > 1024 || contextSize < 1 || contextSize > 1048576 || gpuLayers < 0 || gpuLayers > 1024 || batchSize < 1 || batchSize > 1048576 {
		return nil, fmt.Errorf("invalid llama-server launch arguments")
	}
	args := []string{"-m", modelPath, "--port", port, "--host", "127.0.0.1", "-t", strconv.Itoa(threads), "-c", strconv.Itoa(contextSize), "-ngl", strconv.Itoa(gpuLayers), "-b", strconv.Itoa(batchSize)}
	if !lowRAM {
		args = append(args, "--no-mmap", "--mlock")
	}
	args = append(args, "-fa", "on", "--cont-batching", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0", "--cache-reuse", "256")
	return args, nil
}

// The package-main adapter may be replaced in tests. It starts one reviewed
// OffGrid-owned runtime, detached from the CLI terminal, without a shell.
var runLlamaBackground = func(executable string, args []string) error {
	if strings.TrimSpace(executable) == "" || strings.ContainsRune(executable, 0) {
		return fmt.Errorf("invalid llama-server executable")
	}
	command := exec.Command(executable, args...)
	configureDetachedProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}
