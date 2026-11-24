package inference

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderConnectionHelper(t *testing.T) {
	if os.Getenv("OFFGRID_CONNECTION_HELPER") != "1" {
		return
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, listener.Addr().String())
	// A connected socket can appear in /proc/net/tcp before accept gives it an
	// inode. Exercise that real scheduler window before serving any HTTP bytes.
	time.Sleep(200 * time.Millisecond)
	_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "owned") }))
	os.Exit(0)
}

func TestProviderNamespaceConnectionOwnershipBeforeHTTP(t *testing.T) {
	pin, _ := namespaceFixture(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestProviderConnectionHelper$")
	cmd.Env = append(os.Environ(), "OFFGRID_CONNECTION_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	job, err := startProviderNamespace(context.Background(), cmd, pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := job.close(); err != nil {
			t.Error(err)
		}
	})
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	client, err := endpointHTTPClient(HTTPEndpoint{URL: "http://" + strings.TrimSpace(address), AllowedIPs: []string{"127.0.0.1"}, VerifyConnection: job.verifyConnection})
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + strings.TrimSpace(address))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	var requests atomic.Int32
	unowned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(200) }))
	defer unowned.Close()
	other, err := endpointHTTPClient(HTTPEndpoint{URL: unowned.URL, AllowedIPs: []string{"127.0.0.1"}, VerifyConnection: job.verifyConnection})
	if err != nil {
		t.Fatal(err)
	}
	defer other.CloseIdleConnections()
	if response, err := other.Get(unowned.URL); err == nil {
		response.Body.Close()
		t.Fatal("accepted an unowned listener")
	}
	if requests.Load() != 0 {
		t.Fatal("sent HTTP before socket ownership was verified")
	}
	if err := job.close(); err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("tcp", strings.TrimPrefix(unowned.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if job.verifyConnection(context.Background(), connection) == nil {
		t.Fatal("retired namespace accepted a connection")
	}
}
