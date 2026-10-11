//go:build windows

// Owned, disposable process for native document tests. No files, network,
// user settings, command-line interpreter or production consent override.
package main

func main() { runDocumentFixture() }
