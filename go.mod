module github.com/takuphilchan/offgrid-llm

go 1.21

toolchain go1.21.5

require (
	github.com/go-skynet/go-llama.cpp v0.0.0-20240314183750-6a8041ef6b46
	github.com/shirou/gopsutil/v3 v3.21.11
	golang.org/x/sys v0.20.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
)

replace (
	github.com/tklauser/go-sysconf => github.com/tklauser/go-sysconf v0.3.9
	github.com/tklauser/numcpus => github.com/tklauser/numcpus v0.3.0
)
