package execution

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func isolatedWorkerFixture(t *testing.T, fail string) (*Worker, Binding, *int) {
	t.Helper()
	w, b, root := workerFixture(t)
	p := isolatedPolicyFixture()
	p.Socket = "/run/fixture/docker.sock"
	w.Session.config.Backend = "isolated_linux"
	w.Session.config.Isolation = p
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = TrustedProcessPolicy
	var mu sync.Mutex
	var config isolatedContainerConfig
	id := strings.Repeat("c", 64)
	name := ""
	exists, running, started := false, false, false
	starts := new(int)
	ended := make(chan struct{})
	var end sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/v1.47/info":
			fmt.Fprint(out, `{"ID":"fixture-daemon"}`)
		case r.URL.Path == "/v1.47/containers/create":
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				t.Error(err)
			}
			if len(config.HostConfig.Mounts) != 1 || config.HostConfig.Mounts[0].Source != root || !config.HostConfig.ReadonlyRootfs || config.HostConfig.NetworkMode != "none" {
				t.Error("scope changed", config)
			}
			name = r.URL.Query().Get("name")
			exists = true
			out.WriteHeader(201)
			fmt.Fprintf(out, `{"Id":%q}`, id)
		case strings.HasSuffix(r.URL.Path, "/attach"):
			out.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
			out.WriteHeader(200)
			out.(http.Flusher).Flush()
			mu.Unlock()
			<-ended
			mu.Lock()
			data := []byte("exact — café 日本語\n")
			var header [8]byte
			header[0] = 1
			binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
			out.Write(header[:])
			out.Write(data)
		case strings.HasSuffix(r.URL.Path, "/start"):
			*starts++
			running, started = true, true
			if fail == "start-ack" {
				out.WriteHeader(500)
				return
			}
			out.WriteHeader(204)
			go func() {
				time.Sleep(50 * time.Millisecond)
				mu.Lock()
				running = false
				mu.Unlock()
				end.Do(func() { close(ended) })
			}()
		case strings.HasSuffix(r.URL.Path, "/kill"):
			running = false
			out.WriteHeader(204)
			end.Do(func() { close(ended) })
		case r.Method == "DELETE":
			exists = false
			out.WriteHeader(204)
		case strings.HasSuffix(r.URL.Path, "/json"):
			if !exists {
				out.WriteHeader(404)
				return
			}
			stamp := "0001-01-01T00:00:00Z"
			if started {
				stamp = "2026-10-25T00:00:00Z"
			}
			if fail == "foreign-owner" {
				config.Labels["io.offgrid.executor.ownership"] = strings.Repeat("d", 64)
			}
			fmt.Fprintf(out, `{"Id":%q,"Name":%q,"Config":{"Image":%q,"Labels":{"io.offgrid.executor.ownership":%q}},"State":{"Running":%t,"ExitCode":7,"StartedAt":%q}}`, id, "/"+name, p.Image, config.Labels["io.offgrid.executor.ownership"], running, stamp)
		default:
			t.Error("unexpected Docker operation", r.Method, r.URL)
			out.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Transport = isolationFixtureTransport{next: client.Transport, origin: server.URL}
	w.Processes.isolation = &LinuxIsolation{policy: p, root: root, write: true, daemonID: "fixture-daemon", client: client}
	w.Processes.journal = w.Journal
	return w, b, starts
}

func TestIsolationDurableLifecycleAndUnknownStartNeverReplay(t *testing.T) {
	for _, fail := range []string{"", "start-ack", "identify-persistence", "foreign-owner"} {
		t.Run(fail, func(t *testing.T) {
			w, b, starts := isolatedWorkerFixture(t, fail)
			if fail == "identify-persistence" {
				_, err := w.Journal.db.Exec(`CREATE TRIGGER fixture_fail_container BEFORE UPDATE ON execution_containers BEGIN SELECT RAISE(FAIL,'private state unavailable'); END;`)
				if err != nil {
					t.Fatal(err)
				}
			}
			r := request(w, b, "process_start", "once", ProcessSpec{Program: "go", Arguments: []string{"test", "."}})
			first := w.Handle(context.Background(), r)
			if first.Error != "" {
				t.Fatal(first)
			}
			if fail != "" {
				if first.Receipt.State != "uncertain" {
					t.Fatal(first)
				}
				if fail == "identify-persistence" && *starts != 0 {
					t.Fatal("started before durable ID")
				}
				if fail == "foreign-owner" && *starts != 0 {
					t.Fatal("started foreign ownership")
				}
			} else {
				if first.Receipt.State != "dispatched" {
					t.Fatal(first)
				}
				var p ProcessResult
				if json.Unmarshal(first.Receipt.Result, &p) != nil {
					t.Fatal(first)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				result, err := w.Processes.Wait(ctx, b, p.Handle)
				if err != nil || result.State != "failed" || result.ExitCode == nil || *result.ExitCode != 7 || string(result.Output) != "exact — café 日本語\n" || result.Directory != "/workspace" {
					t.Fatal(result, err)
				}
				w.finishing.Wait()
			}
			before := *starts
			second := w.Handle(context.Background(), r)
			if second.Error != "" || *starts != before {
				t.Fatal("replayed", second, *starts)
			}
			if err := w.Processes.isolation.recover(context.Background(), w.Journal); err != nil {
				if fail == "foreign-owner" {
					return
				}
				t.Fatal("cleanup", err)
			}
			remaining, err := w.Journal.ownedContainers(context.Background())
			if err != nil || len(remaining) != 0 {
				t.Fatal(remaining, err)
			}
		})
	}
}

func TestRealIsolationWorkerCrashCleanup(t *testing.T) {
	image := os.Getenv("OFFGRID_TEST_ISOLATION_IMAGE")
	if image == "" || runtime.GOOS != "linux" {
		t.Skip("explicit installed Linux image required")
	}
	// WSL mounted Windows temp folders may not implement private Unix modes.
	// Exercise the real journal on native Linux storage rather than weakening
	// the account-private state requirement for that filesystem.
	t.Setenv("TMPDIR", "/tmp")
	s, b, root := fixture(t)
	p := isolatedPolicyFixture()
	p.Image = image
	p.UID, p.GID = os.Getuid(), os.Getgid()
	if socket := os.Getenv("OFFGRID_TEST_ISOLATION_SOCKET"); socket != "" {
		p.Socket = socket
	}
	s.config.Backend = "isolated_linux"
	s.config.Isolation = p
	s.config.Permissions.Processes = true
	i, err := NewLinuxIsolation(context.Background(), p, root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer i.client.CloseIdleConnections()
	directory := t.TempDir()
	if err = ProtectPrivatePath(directory); err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if j != nil {
			j.Close()
		}
	}()
	owner, _ := opaqueToken()
	c := containerOwnership{Operation: "crash-case", Daemon: i.daemonID, Name: "offgrid-" + owner, Owner: owner}
	r := Request{Version: ProtocolVersion, Binding: b, Operation: "process_start", OperationID: c.Operation, ID: c.Operation, Arguments: json.RawMessage(`{"program":"sh","arguments":["-c","sleep 30"],"timeout_seconds":0}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, fresh, err := j.Claim(ctx, r.OperationID, r.Payload()); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if err = j.claimContainer(ctx, r, c); err != nil {
		t.Fatal(err)
	}
	config, err := p.containerConfig(root, ".", owner, true, ProcessSpec{Program: "sh", Arguments: []string{"-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	status, data, err := i.request(ctx, "POST", "/containers/create?name="+c.Name, config)
	if err != nil || status != 201 {
		t.Fatal(status, err)
	}
	var created struct {
		ID string `json:"Id"`
	}
	json.Unmarshal(data, &created)
	c.ID = created.ID
	// The defer is an independent best-effort cleanup only for this exact owned
	// fixture object if an assertion fails; it never targets installed workloads.
	defer i.cleanup(context.Background(), c)
	if err = j.identifyContainer(ctx, c); err != nil {
		t.Fatal(err)
	}
	status, _, err = i.request(ctx, "POST", "/containers/"+c.ID+"/start", nil)
	if err != nil || status != 204 {
		t.Fatal(status, err)
	}
	// Even removal of the disposable host directory must not prevent stopping
	// the exact owned container. No workspace content is needed for cleanup.
	if err = os.Remove(root); err != nil {
		t.Fatal(err)
	}
	j.Close() // Simulated worker death: no model retry and no process adoption.
	j = nil
	if err = cleanupStoppedWorker(s.config, directory); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := i.inspect(ctx, c); err != nil || exists {
		t.Fatal("orphan survived", exists, err)
	}
	j2, err := OpenJournal(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	receipt, fresh, err := j2.Claim(ctx, r.OperationID, r.Payload())
	if err != nil || fresh || receipt.State != "uncertain" {
		t.Fatal("crash became replay/success", receipt, fresh, err)
	}
}

func TestIsolationFramingBoundsAndMalformedStreams(t *testing.T) {
	for _, data := range [][]byte{{1}, {3, 0, 0, 0, 0, 0, 0, 0}, {1, 1, 0, 0, 0, 0, 0, 0}, {1, 0, 0, 0, 0, 0, 0, 2, 'x'}} {
		if copyContainerOutput(bytes.NewReader(data), &processOutput{}) == nil {
			t.Fatal("accepted malformed stream", data)
		}
	}
	var payload bytes.Buffer
	var header [8]byte
	header[0] = 2
	binary.BigEndian.PutUint32(header[4:], MaxProcessOutput+1024)
	payload.Write(header[:])
	payload.Write(bytes.Repeat([]byte("a"), MaxProcessOutput+1024))
	var output processOutput
	if err := copyContainerOutput(&payload, &output); err != nil {
		t.Fatal(err)
	}
	data, truncated := output.snapshot()
	if len(data) != MaxProcessOutput || !truncated {
		t.Fatal(len(data), truncated)
	}
}

// Opt-in isolated real effects: only disposable folders, the explicitly
// supplied already-installed image, no pulls and no installed-service changes.
func TestRealIsolatedLinuxCommandLifecycle(t *testing.T) {
	image := os.Getenv("OFFGRID_TEST_ISOLATION_IMAGE")
	if image == "" || runtime.GOOS != "linux" {
		t.Skip("explicit installed Linux image required")
	}
	for run := 0; run < 3; run++ {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			w, b, root := workerFixture(t)
			p := isolatedPolicyFixture()
			p.Image = image
			p.UID, p.GID = os.Getuid(), os.Getgid()
			if socket := os.Getenv("OFFGRID_TEST_ISOLATION_SOCKET"); socket != "" {
				p.Socket = socket
			}
			w.Session.config.Backend = "isolated_linux"
			w.Session.config.Isolation = p
			w.Session.config.Permissions.Processes = true
			m, err := NewIsolatedProcessManager(context.Background(), w.Session, w.Journal)
			if err != nil {
				t.Fatal(err)
			}
			w.Processes = m
			t.Cleanup(func() { m.Close() })
			w.AuthorizeProcess = TrustedProcessPolicy
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			r := request(w, b, "process_start", "unicode", ProcessSpec{Program: "sh", Arguments: []string{"-c", `test ! -S /var/run/docker.sock && test -z "$OFFGRID_EXECUTOR_TOKEN" && printf 'OffGrid — café 日本語 12345' > result.txt && cat result.txt`}})
			r.Approved = true
			first := w.Handle(ctx, r)
			if first.Error != "" || first.Receipt == nil || first.Receipt.State != "dispatched" {
				t.Fatal(first)
			}
			var started ProcessResult
			json.Unmarshal(first.Receipt.Result, &started)
			result, err := m.Wait(ctx, b, started.Handle)
			if err != nil || result.State != "exited" || result.ExitCode == nil || *result.ExitCode != 0 || string(result.Output) != "OffGrid — café 日本語 12345" {
				t.Fatal(result, err)
			}
			data, err := os.ReadFile(filepath.Join(root, "result.txt"))
			if err != nil || string(data) != string(result.Output) {
				t.Fatal("independent readback", string(data), err)
			}
			w.finishing.Wait()
			again := w.Handle(ctx, r)
			if again.Receipt == nil || again.Receipt.State != "complete" {
				t.Fatal(again)
			}
			for _, mode := range []string{"stop", "timeout", "failed"} {
				spec := ProcessSpec{Program: "sh", Arguments: []string{"-c", "sleep 30"}}
				if mode == "timeout" {
					spec.TimeoutSeconds = 1
				}
				if mode == "failed" {
					spec.Arguments = []string{"-c", "exit 9"}
				}
				req := request(w, b, "process_start", mode, spec)
				req.Approved = true
				response := w.Handle(ctx, req)
				if response.Receipt == nil || response.Receipt.State != "dispatched" {
					t.Fatal(response)
				}
				var proc ProcessResult
				json.Unmarshal(response.Receipt.Result, &proc)
				if mode == "stop" {
					result, err = m.Stop(ctx, b, proc.Handle)
				} else {
					result, err = m.Wait(ctx, b, proc.Handle)
				}
				expect := map[string]string{"stop": "cancelled", "timeout": "timed_out", "failed": "failed"}[mode]
				if err != nil || result.State != expect || result.ExitCode == nil {
					t.Fatal(mode, result, err)
				}
			}
			w.finishing.Wait()
			remaining, err := w.Journal.ownedContainers(ctx)
			if err != nil || len(remaining) != 0 {
				t.Fatal("owned container leak", remaining, err)
			}
		})
	}
}
