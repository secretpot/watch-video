package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

type mcpTestReply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *mcpError       `json:"error"`
}

func mcpTestWait[T any](t *testing.T, value <-chan T) T {
	t.Helper()
	select {
	case got := <-value:
		return got
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for MCP stdio")
		var zero T
		return zero
	}
}

func mcpTestSession(t *testing.T, call func(context.Context, string, json.RawMessage) ([]mcpContent, error)) (func(string), func() mcpTestReply, func()) {
	t.Helper()
	serverInput, clientOutput := io.Pipe()
	clientInput, serverOutput := io.Pipe()
	tools := []mcpTool{{Name: "example", Description: "Return evidence", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveMCP(t.Context(), serverInput, serverOutput, tools, call)
		serverOutput.Close()
	}()
	t.Cleanup(func() {
		clientOutput.Close()
		clientInput.Close()
	})
	replies := make(chan mcpTestReply, 1)
	go func() {
		defer close(replies)
		decoder := json.NewDecoder(clientInput)
		for {
			var reply mcpTestReply
			if decoder.Decode(&reply) != nil {
				return
			}
			select {
			case replies <- reply:
			case <-t.Context().Done():
				return
			}
		}
	}()
	send := func(message string) {
		t.Helper()
		if _, err := fmt.Fprintln(clientOutput, message); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() mcpTestReply {
		t.Helper()
		reply := mcpTestWait(t, replies)
		if reply.JSONRPC != "2.0" {
			t.Fatalf("invalid or missing JSON-RPC response: %+v", reply)
		}
		return reply
	}
	shutdown := func() {
		t.Helper()
		clientOutput.Close()
		if err := mcpTestWait(t, serverDone); err != nil {
			t.Fatal(err)
		}
	}
	return send, receive, shutdown
}

func TestMCPStdioExchangesTextImagesAndErrors(t *testing.T) {
	frame := []byte{0xff, 0xd8, 0xff, 0xd9}
	send, receive, shutdown := mcpTestSession(t, func(_ context.Context, _ string, arguments json.RawMessage) ([]mcpContent, error) {
		if string(arguments) == `{"fail":true}` {
			return nil, errors.New("media could not be opened")
		}
		return []mcpContent{{Type: "text", Text: "00:16\ncaption"}, {Type: "image", Data: frame, MimeType: "image/jpeg"}}, nil
	})
	send(`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"future","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	reply := receive()
	var initialization struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(reply.Result, &initialization); err != nil || string(reply.ID) != `"init"` ||
		initialization.ProtocolVersion != mcpProtocolVersion || string(initialization.Capabilities["tools"]) != "{}" {
		t.Fatalf("initialize: %s, %v", reply.Result, err)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	reply = receive()
	var list struct{ Tools []mcpTool }
	if err := json.Unmarshal(reply.Result, &list); err != nil || string(reply.ID) != "1" ||
		len(list.Tools) != 1 || list.Tools[0].Name != "example" || string(list.Tools[0].InputSchema) != `{"type":"object"}` {
		t.Fatalf("tools/list: %s, %v", reply.Result, err)
	}
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"example"}}`)
	reply = receive()
	var result mcpToolResult
	if err := json.Unmarshal(reply.Result, &result); err != nil || string(reply.ID) != "2" || result.IsError ||
		len(result.Content) != 2 || result.Content[0].Text != "00:16\ncaption" ||
		result.Content[1].Type != "image" || result.Content[1].MimeType != "image/jpeg" || !bytes.Equal(result.Content[1].Data, frame) {
		t.Fatalf("tools/call: %s, %v", reply.Result, err)
	}
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"example","arguments":{"fail":true}}}`)
	reply = receive()
	if err := json.Unmarshal(reply.Result, &result); err != nil || string(reply.ID) != "3" || !result.IsError ||
		len(result.Content) != 1 || result.Content[0].Text != "media could not be opened" {
		t.Fatalf("execution error: %s, %v", reply.Result, err)
	}
	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"missing"}}`)
	if reply := receive(); string(reply.ID) != "4" || reply.Error == nil || reply.Error.Code != -32602 || len(reply.Result) != 0 {
		t.Fatalf("protocol error: %+v", reply)
	}
	shutdown()
}

func TestMCPStdioCancelsWorkWithoutBlockingPingOrEOF(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{}, 1)
	send, receive, shutdown := mcpTestSession(t, func(ctx context.Context, _ string, _ json.RawMessage) ([]mcpContent, error) {
		started <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	})
	send(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	reply := receive()
	if !bytes.Contains(reply.Result, []byte(`"protocolVersion":"2025-06-18"`)) {
		t.Fatalf("expected the requested supported version, got %s", reply.Result)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":"slow","method":"tools/call","params":{"name":"example"}}`)
	mcpTestWait(t, started)
	send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if reply := receive(); string(reply.ID) != "1" || string(reply.Result) != "{}" {
		t.Fatalf("ping while a tool runs: %+v", reply)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"slow"}}`)
	mcpTestWait(t, cancelled)
	send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if reply := receive(); string(reply.ID) != "2" || string(reply.Result) != "{}" {
		t.Fatalf("ping after cancellation: %+v", reply)
	}
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"example"}}`)
	mcpTestWait(t, started)
	shutdown()
	mcpTestWait(t, cancelled)
}
