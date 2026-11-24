package inference

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestONNXNamespaceProtocolHelper(t *testing.T) {
	if os.Getenv("OFFGRID_ONNX_NAMESPACE_HELPER") != "1" {
		return
	}
	child := namespaceFixtureCommand("child")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	connection, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: os.Getenv("OFFGRID_NAMESPACE_SOCKET"), Net: "unixgram"})
	if err != nil {
		os.Exit(3)
	}
	_, err = connection.Write([]byte("parent"))
	connection.Close()
	if err != nil {
		os.Exit(4)
	}
	onnxProtocolWorker()
}

func TestONNXProcessProtocolRetiresOwnedNamespaceTree(t *testing.T) {
	pin, connection := namespaceFixture(t)
	descriptor := onnxDescriptorFixture()
	raw, _ := json.Marshal(descriptor)
	start := func(command *exec.Cmd) (func(*exec.Cmd) error, error) {
		job, err := startProviderNamespace(context.Background(), command, pin)
		if err != nil {
			return nil, err
		}
		return func(*exec.Cmd) error { return job.close() }, nil
	}
	provider, err := startProcessProviderUsing(context.Background(), os.Args[0], []string{"-test.run=^TestONNXNamespaceProtocolHelper$"}, descriptor, append(os.Environ(), "OFFGRID_ONNX_NAMESPACE_HELPER=1", "OFFGRID_ONNX_PROTOCOL_DESCRIPTOR="+string(raw)), "", start)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	children := namespaceFixtureChildren(t, connection)
	request := ProviderRequest{RequestID: "owned", CancellationID: "owned", ModelDigest: descriptor.ModelDigest, ContextLength: 8, Prompt: json.RawMessage(`{"input":["first","second"]}`)}
	if err := provider.Generate(context.Background(), request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	assertNamespaceChildrenExited(t, children)
}

func TestONNXNamespacePreStartRejectionClosesAllPipeEnds(t *testing.T) {
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for range 32 {
		_, err := startProcessProviderUsing(context.Background(), os.Args[0], nil, onnxDescriptorFixture(), []string{}, "", func(*exec.Cmd) (func(*exec.Cmd) error, error) {
			return nil, errors.New("fixture rejected before Start")
		})
		if err == nil {
			t.Fatal("starter rejection lost")
		}
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before)+1 {
		t.Fatalf("pre-start failure leaked pipe ends: before=%d after=%d", len(before), len(after))
	}
}
