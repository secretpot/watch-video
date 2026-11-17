package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

const mcpProtocolVersion = "2025-11-25"
const watchVideoVersion = "0.4.1"

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type mcpContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func (content mcpContent) MarshalJSON() ([]byte, error) {
	if content.Type == "text" {
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{content.Type, content.Text})
	}
	type wireContent mcpContent
	return json.Marshal(wireContent(content))
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpLine struct {
	text string
	err  error
}

func readMCPLines(ctx context.Context, input io.Reader, lines chan<- mcpLine) {
	defer close(lines)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		select {
		case lines <- mcpLine{text: scanner.Text()}:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case lines <- mcpLine{err: err}:
		case <-ctx.Done():
		}
	}
}

func mcpIDKey(id json.RawMessage) string {
	if len(id) == 0 {
		return ""
	}
	if id[0] == '"' {
		var value string
		if json.Unmarshal(id, &value) == nil {
			return "s" + value
		}
		return ""
	}
	digits := strings.TrimPrefix(string(id), "-")
	if digits == "" || strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return ""
	}
	if digits == "0" {
		return "n0"
	}
	return "n" + string(id)
}

func serveMCP(ctx context.Context, input io.Reader, output io.Writer, tools []mcpTool, call func(context.Context, string, json.RawMessage) ([]mcpContent, error)) error {
	ctx, stop := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		stop()
		if reader, ok := input.(io.Closer); ok {
			reader.Close()
		}
		workers.Wait()
	}()

	lines := make(chan mcpLine)
	go readMCPLines(ctx, input, lines)
	type pendingCall struct {
		context context.Context
		cancel  context.CancelFunc
	}
	type finishedCall struct {
		id     json.RawMessage
		key    string
		result mcpToolResult
	}
	pending := make(map[string]pendingCall)
	finished := make(chan finishedCall)
	encoder := json.NewEncoder(output)
	initialized := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case done := <-finished:
			request := pending[done.key]
			delete(pending, done.key)
			err := request.context.Err()
			request.cancel()
			if err == nil {
				if err := encoder.Encode(mcpResponse{JSONRPC: "2.0", ID: done.id, Result: done.result}); err != nil {
					return err
				}
			}
		case line, open := <-lines:
			if !open {
				return nil
			}
			if line.err != nil {
				return line.err
			}
			var request mcpRequest
			decodeErr := json.Unmarshal([]byte(line.text), &request)
			key := mcpIDKey(request.ID)
			response := mcpResponse{JSONRPC: "2.0", ID: request.ID}
			var syntaxErr *json.SyntaxError
			switch {
			case errors.As(decodeErr, &syntaxErr):
				response.ID = nil
				response.Error = &mcpError{-32700, "Parse error"}
			case decodeErr != nil || request.JSONRPC != "2.0" || request.Method == "" || (request.ID != nil && key == ""):
				if key == "" {
					response.ID = nil
				}
				response.Error = &mcpError{-32600, "Invalid request"}
			case request.ID == nil:
				if request.Method == "notifications/cancelled" {
					var params struct {
						RequestID json.RawMessage `json:"requestId"`
					}
					if json.Unmarshal(request.Params, &params) == nil {
						if work, ok := pending[mcpIDKey(params.RequestID)]; ok {
							work.cancel()
						}
					}
				}
				continue
			case len(request.Params) != 0 && request.Params[0] != '{':
				response.Error = &mcpError{-32602, "Expected params to be an object"}
			case request.Method == "ping":
				response.Result = struct{}{}
			case request.Method == "initialize":
				var params struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				if initialized || json.Unmarshal(request.Params, &params) != nil || params.ProtocolVersion == "" {
					response.Error = &mcpError{-32602, "Expected one initialize request with protocolVersion"}
					break
				}
				version := params.ProtocolVersion
				switch version {
				case "2024-11-05", "2025-03-26", "2025-06-18", mcpProtocolVersion:
				default:
					version = mcpProtocolVersion
				}
				response.Result = map[string]any{
					"protocolVersion": version,
					"capabilities":    map[string]any{"tools": struct{}{}},
					"serverInfo":      map[string]string{"name": "watch-video", "version": watchVideoVersion},
				}
				initialized = true
			case !initialized:
				response.Error = &mcpError{-32002, "Server not initialized"}
			case request.Method == "tools/list":
				response.Result = struct {
					Tools []mcpTool `json:"tools"`
				}{tools}
			case request.Method == "tools/call":
				var params struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				if json.Unmarshal(request.Params, &params) != nil || params.Name == "" ||
					(len(params.Arguments) != 0 && params.Arguments[0] != '{') {
					response.Error = &mcpError{-32602, "Expected a tool name and optional arguments object"}
					break
				}
				known := false
				for _, tool := range tools {
					if tool.Name == params.Name {
						known = true
						break
					}
				}
				if !known {
					response.Error = &mcpError{-32602, "Unknown tool: " + params.Name}
					break
				}
				if _, exists := pending[key]; exists {
					response.Error = &mcpError{-32600, "Request ID already in use"}
					break
				}
				if params.Arguments == nil {
					params.Arguments = json.RawMessage("{}")
				}
				workCtx, cancel := context.WithCancel(ctx)
				pending[key] = pendingCall{workCtx, cancel}
				workers.Go(func() {
					content, err := call(workCtx, params.Name, params.Arguments)
					if err != nil {
						content = []mcpContent{{Type: "text", Text: err.Error()}}
					} else if content == nil {
						content = []mcpContent{}
					}
					done := finishedCall{request.ID, key, mcpToolResult{content, err != nil}}
					select {
					case finished <- done:
					case <-ctx.Done():
					}
				})
				continue
			default:
				response.Error = &mcpError{-32601, "Method not found"}
			}
			if err := encoder.Encode(response); err != nil {
				return err
			}
		}
	}
}
