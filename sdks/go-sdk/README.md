# Nexus Go SDK

Minimal Go SDK for Nexus Core API.

## Installation

```bash
go get github.com/nexus-io/go-sdk
```

## Quick Start

```go
package main

import (
  "context"
  "fmt"

  nexus "github.com/nexus-io/go-sdk"
)

func main() {
  client := nexus.NewClient("http://localhost:8080", "dev-api-key")

  tools, err := client.ListTools(context.Background())
  if err != nil {
    panic(err)
  }
  fmt.Printf("tools: %d\n", len(tools))

  run, err := client.RunTool(context.Background(), nexus.RunRequest{
    ToolName: "echo",
    Input: map[string]any{"message": "hello"},
  })
  if err != nil {
    panic(err)
  }
  fmt.Printf("status=%s decision=%s\n", run.Status, run.Decision)
}
```

## API

- `NewClient(baseURL, apiKey string) *Client`
- `(*Client) ListTools(ctx context.Context) ([]Tool, error)`
- `(*Client) RunTool(ctx context.Context, req RunRequest) (*RunResponse, error)`

## Configuration

`Client` fields are public if you need advanced control:

- `BaseURL`
- `APIKey`
- `HTTP` (`*http.Client`)
