package agents

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func TestMCPServerNegotiatesListsAndCallsTools(t *testing.T) {
	registry := NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(capabilities.DefaultPolicy{}))
	server := registry.MCPServer("test")
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "contract-test", Version: "test"}, nil)
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	}()

	listed, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	foundCalculator := false
	for _, tool := range listed.Tools {
		if tool.Name == "calculator" {
			foundCalculator = true
			break
		}
	}
	if !foundCalculator {
		t.Fatal("calculator missing from negotiated MCP tool list")
	}
	result, err := clientSession.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "calculator", Arguments: map[string]interface{}{"expression": "2+2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("unexpected MCP tool result: %#v", result)
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok || text.Text != "4" {
		t.Fatalf("unexpected calculator content: %#v", result.Content)
	}
}
