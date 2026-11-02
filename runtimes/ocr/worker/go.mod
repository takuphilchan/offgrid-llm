module github.com/takuphilchan/offgrid-llm/runtimes/ocr/worker

go 1.26.0

toolchain go1.26.6

require (
	github.com/takuphilchan/offgrid-llm v0.0.0
	github.com/tetratelabs/wazero v1.12.0
)

require golang.org/x/sys v0.44.0 // indirect

replace github.com/takuphilchan/offgrid-llm => ../../..
