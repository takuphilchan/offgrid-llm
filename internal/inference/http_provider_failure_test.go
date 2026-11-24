package inference

import (
	"context"
	"errors"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPProviderFactoryFailureReturnsNoRuntimeToScheduler(t *testing.T) {
	endpoint, released := httpProviderFixture(t, ProviderOllama, func(http.ResponseWriter, *http.Request) { t.Error("generation dispatched") })
	endpoint.VerifyConnection = func(context.Context, net.Conn) error { return errors.New("unowned peer") }
	routes, err := NewHTTPEndpointRoutes(map[string]HTTPEndpoint{"failure@r1": endpoint})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewProviderRegistry()
	if err := routes.Register(registry); err != nil {
		t.Fatal(err)
	}
	if runtime, err := registry.Open(context.Background(), endpoint.Descriptor); err == nil || runtime != nil {
		t.Fatal("failed factory returned a typed nil runtime", runtime, err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if err := scheduler.Generate(context.Background(), endpoint.Descriptor, httpProviderRequest(t, endpoint, nil), func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("scheduler admitted failed HTTP startup")
	}
	if released.Load() != 2 {
		t.Fatal("failed startup did not release each lease exactly once", released.Load())
	}
}

func TestHTTPProviderRemoteResourceDomainRejectsLocalInterface(t *testing.T) {
	local := []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.4.2"), Mask: net.CIDRMask(24, 32)}, &net.IPNet{IP: net.ParseIP("2001:db8::5"), Mask: net.CIDRMask(64, 128)}}
	for _, address := range []string{"192.168.4.2", "2001:db8::5"} {
		if rejectLocalEndpointAddresses([]string{address}, local) == nil {
			t.Fatal("local resource reported remote", address)
		}
	}
	if err := rejectLocalEndpointAddresses([]string{"192.168.4.3"}, local); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPProviderRejectsMalformedTerminalEvidence(t *testing.T) {
	cases := map[string]string{
		"ambiguous":       `{"model":"fixture-model","model":"fixture-model","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":1}`,
		"missing usage":   `{"model":"fixture-model","done":true,"done_reason":"stop"}`,
		"negative usage":  `{"model":"fixture-model","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":-1}`,
		"exceeded budget": `{"model":"fixture-model","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":65}`,
		"changed model":   `{"model":"other","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":1}`,
		"cloud forwarded": `{"model":"fixture-model","remote_host":"https://cloud.invalid","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":1}`,
		"late output":     "{\"model\":\"fixture-model\",\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":2,\"eval_count\":1}\n{\"model\":\"fixture-model\",\"message\":{\"content\":\"late\"}}",
		"oversized":       strings.Repeat("x", 2<<20),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			endpoint, _ := httpProviderFixture(t, ProviderOllama, func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, body) })
			p, err := NewHTTPProvider(context.Background(), endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			var completed bool
			err = p.Generate(context.Background(), httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error { completed = completed || event.Type == ProviderEventDone; return nil })
			if err == nil || completed {
				t.Fatalf("invalid terminal accepted: %v %v", completed, err)
			}
		})
	}
}

func TestHTTPProvidersValidateCompleteToolDecisions(t *testing.T) {
	tools := []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "read", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}, "additionalProperties": false}}}}
	for _, id := range []ProviderID{ProviderOllama, ProviderVLLM} {
		for _, valid := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s-valid-%v", id, valid), func(t *testing.T) {
				arguments := `{"path":"x"}`
				if !valid {
					arguments = `{"path":42}`
				}
				endpoint, _ := httpProviderFixture(t, id, func(w http.ResponseWriter, r *http.Request) {
					if id == ProviderOllama {
						fmt.Fprintf(w, "{\"model\":\"fixture-model\",\"message\":{\"tool_calls\":[{\"function\":{\"name\":\"read\",\"arguments\":%s}}]},\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":3,\"eval_count\":2}\n", arguments)
					} else {
						fmt.Fprintf(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":%q}}]}}]}\n\n", arguments)
						fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"model\":\"fixture-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")
					}
				})
				p, err := NewHTTPProvider(context.Background(), endpoint)
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				delivered := false
				err = p.Generate(context.Background(), httpProviderRequest(t, endpoint, tools), func(event ProviderEvent) error { delivered = delivered || event.Type == ProviderEventTool; return nil })
				if valid && (err != nil || !delivered) {
					t.Fatalf("valid tool rejected: %v %v", delivered, err)
				}
				if !valid && (err == nil || delivered) {
					t.Fatalf("invalid tool delivered: %v %v", delivered, err)
				}
			})
		}
	}
}

func TestHTTPProviderRedirectDoesNotForwardCredentials(t *testing.T) {
	var escaped atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { escaped.Add(1) }))
	defer sink.Close()
	endpoint, _ := httpProviderFixture(t, ProviderVLLM, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	})
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	err = p.Generate(context.Background(), httpProviderRequest(t, endpoint, nil), func(ProviderEvent) error { return nil })
	if err == nil || escaped.Load() != 0 {
		t.Fatalf("redirect escaped=%d err=%v", escaped.Load(), err)
	}
}

func TestHTTPProviderRejectsOverLimitPaddingAfterTerminal(t *testing.T) {
	for _, id := range []ProviderID{ProviderOllama, ProviderVLLM} {
		t.Run(string(id), func(t *testing.T) {
			endpoint, _ := httpProviderFixture(t, id, func(w http.ResponseWriter, r *http.Request) {
				if id == ProviderOllama {
					fmt.Fprintln(w, `{"model":"fixture-model","done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":1}`)
				} else {
					fmt.Fprint(w, "data: {\"model\":\"fixture-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"model\":\"fixture-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n\ndata: [DONE]\n\n")
				}
				padding := strings.Repeat("\n", 64<<10)
				if id == ProviderVLLM {
					padding = strings.Repeat(": padding\n", 8192)
				}
				for n := 0; n < 600; n++ {
					if _, err := fmt.Fprint(w, padding); err != nil {
						return
					}
				}
			})
			p, err := NewHTTPProvider(context.Background(), endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			terminal := false
			err = p.Generate(context.Background(), httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error {
				terminal = terminal || event.Type == ProviderEventDone || event.Type == ProviderEventTool
				return nil
			})
			if err == nil || terminal {
				t.Fatalf("over-limit stream completed=%v error=%v", terminal, err)
			}
		})
	}
}

func TestHTTPProviderRejectsCloudAliasBeforePrompt(t *testing.T) {
	var prompts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			fmt.Fprint(w, `{"version":"fixture-1"}`)
		case "/api/tags":
			fmt.Fprintf(w, `{"models":[{"name":"fixture-model","digest":%q,"remote_model":"upstream","remote_host":"https://cloud.invalid"}]}`, strings.Repeat("a", 64))
		default:
			prompts.Add(1)
		}
	}))
	defer server.Close()
	endpoint, _ := httpProviderFixture(t, ProviderOllama, func(http.ResponseWriter, *http.Request) {})
	endpoint.URL = server.URL
	if p, err := NewHTTPProvider(context.Background(), endpoint); err == nil {
		p.Close()
		t.Fatal("cloud alias admitted")
	}
	if prompts.Load() != 0 {
		t.Fatal("prompt sent before cloud alias rejection")
	}
}
