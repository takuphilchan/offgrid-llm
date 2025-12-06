#!/usr/bin/env python3
"""
Simple MCP Test Server for OffGrid LLM

This is a minimal MCP server that provides test tools for integration testing.
Run with: python3 mcp_test_server.py

The server listens on http://localhost:3100 and provides these tools:
- echo: Returns the input text
- add_numbers: Adds two numbers
- get_weather: Returns mock weather data
"""

from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import sys

class MCPHandler(BaseHTTPRequestHandler):
    """Handler for MCP JSON-RPC requests"""
    
    # Available tools
    TOOLS = [
        {
            "name": "echo",
            "description": "Echo back the input text. Useful for testing.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "The text to echo back"
                    }
                },
                "required": ["text"]
            }
        },
        {
            "name": "add_numbers",
            "description": "Add two numbers together and return the result.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "a": {
                        "type": "number",
                        "description": "First number"
                    },
                    "b": {
                        "type": "number",
                        "description": "Second number"
                    }
                },
                "required": ["a", "b"]
            }
        },
        {
            "name": "get_weather",
            "description": "Get the current weather for a city (mock data).",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "city": {
                        "type": "string",
                        "description": "The city name"
                    }
                },
                "required": ["city"]
            }
        }
    ]
    
    def do_POST(self):
        # Read request body
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)
        
        try:
            request = json.loads(body)
        except json.JSONDecodeError:
            self.send_error_response(-32700, "Parse error")
            return
        
        # Extract JSON-RPC fields
        request_id = request.get("id", 1)
        method = request.get("method", "")
        params = request.get("params", {})
        
        print(f"[MCP] Method: {method}, Params: {params}", file=sys.stderr)
        
        # Handle methods
        if method == "initialize":
            result = {
                "protocolVersion": "2024-11-05",
                "capabilities": {
                    "tools": {}
                },
                "serverInfo": {
                    "name": "offgrid-test-mcp",
                    "version": "1.0.0"
                }
            }
        elif method == "tools/list":
            result = {"tools": self.TOOLS}
        elif method == "tools/call":
            result = self.call_tool(params)
        else:
            self.send_error_response(-32601, f"Method not found: {method}", request_id)
            return
        
        self.send_success_response(result, request_id)
    
    def call_tool(self, params):
        """Execute a tool call"""
        name = params.get("name", "")
        args = params.get("arguments", {})
        
        try:
            if name == "echo":
                text = args.get("text", "")
                return {
                    "content": [{"type": "text", "text": f"Echo: {text}"}]
                }
            elif name == "add_numbers":
                a = float(args.get("a", 0))
                b = float(args.get("b", 0))
                result = a + b
                return {
                    "content": [{"type": "text", "text": f"Result: {a} + {b} = {result}"}]
                }
            elif name == "get_weather":
                city = args.get("city", "Unknown")
                # Mock weather data
                return {
                    "content": [{
                        "type": "text", 
                        "text": f"Weather in {city}: Sunny, 22°C, Humidity 45%"
                    }]
                }
            else:
                return {
                    "content": [{"type": "text", "text": f"Unknown tool: {name}"}],
                    "isError": True
                }
        except Exception as e:
            return {
                "content": [{"type": "text", "text": f"Tool error: {str(e)}"}],
                "isError": True
            }
    
    def send_success_response(self, result, request_id):
        """Send a successful JSON-RPC response"""
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": result
        }
        self.send_json_response(response)
    
    def send_error_response(self, code, message, request_id=1):
        """Send an error JSON-RPC response"""
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "error": {
                "code": code,
                "message": message
            }
        }
        self.send_json_response(response)
    
    def send_json_response(self, data):
        """Send a JSON response"""
        body = json.dumps(data).encode('utf-8')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', len(body))
        self.end_headers()
        self.wfile.write(body)
    
    def log_message(self, format, *args):
        """Log to stderr"""
        print(f"[MCP Server] {args[0]}", file=sys.stderr)


def main():
    port = 3100
    server = HTTPServer(('localhost', port), MCPHandler)
    print(f"MCP Test Server running on http://localhost:{port}", file=sys.stderr)
    print("Available tools: echo, add_numbers, get_weather", file=sys.stderr)
    print("Press Ctrl+C to stop", file=sys.stderr)
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...", file=sys.stderr)
        server.shutdown()


if __name__ == "__main__":
    main()
