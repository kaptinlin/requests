# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the `requests` package.

## Project Overview

**Module**: `github.com/kaptinlin/requests`
**Go Version**: 1.26
**Purpose**: HTTP client library providing a fluent/builder pattern interface for making HTTP requests with middleware, retry mechanisms, streaming, and multiple encoding formats.

The `requests` library simplifies HTTP operations in Go by offering an ergonomic alternative to `net/http` with features like automatic retries, middleware chains, streaming support, and built-in encoders/decoders for JSON, XML, and YAML.

## Design Philosophy

- **Fluent Builder Pattern**: All request configuration uses method chaining for readability and ease of use
- **Middleware-First Architecture**: Extensible request/response processing pipeline for cross-cutting concerns
- **Zero-Panic Policy**: Library code returns errors instead of panicking; callers control error handling
- **Memory Efficiency**: Uses buffer pooling (`valyala/bytebufferpool`) for high-throughput scenarios
- **Modern Go Features**: Leverages Go 1.26 features (iterators, Swiss Tables, maps/slices packages)
- **Pluggable Components**: Strategy pattern for retry logic, auth methods, and encoders/decoders

## Commands

### Primary Targets
```bash
task test          # Run tests with race detection
task lint          # Run golangci-lint v2.9.0 and go mod tidy checks
task verify        # Run verify (deps, fmt, vet, lint, test)
task clean         # Clean build artifacts and caches
```

### Development Commands
```bash
go test ./...                    # Run tests without race detection
go test -v ./...                 # Run tests with verbose output
go test -bench=. -benchmem ./... # Run benchmarks
golangci-lint run                # Run linter directly
```

## Architecture

### Core Components

```
requests/
├── client.go           # Client struct, configuration, HTTP/2 setup
├── request.go          # RequestBuilder (fluent API for building requests)
├── response.go         # Response wrapper with streaming and parsing
├── auth.go             # Authentication methods (Basic, Bearer, API Key)
├── retry.go            # Retry logic with backoff strategies
├── middleware.go       # Middleware interface and chain execution
├── stream.go           # Streaming callbacks and buffer management
├── coder.go            # Encoder/Decoder interfaces
├── json.go             # JSON encoding/decoding (go-json-experiment)
├── xml.go              # XML encoding/decoding
├── yaml.go             # YAML encoding/decoding (goccy/go-yaml)
├── form.go             # Form and multipart form handling
├── pool.go             # Buffer pooling for memory efficiency
├── logger.go           # Logger interface
├── errors.go           # Sentinel errors
└── middlewares/        # Built-in middleware implementations
    ├── header.go       # Header manipulation
    ├── cache.go        # Response caching
    └── cookie.go       # Cookie management
```

### Request Flow

1. **Client Creation**: `requests.URL()` or `requests.Create()` initializes client with config
2. **Request Building**: `client.Get("/path")` returns `RequestBuilder` for method chaining
3. **Middleware Chain**: Client and request-level middlewares wrap the request
4. **Retry Logic**: Failed requests retry with backoff strategy if configured
5. **Response Handling**: Response body is buffered or streamed based on configuration

### Key Types and Interfaces

**Client**
```go
type Client struct {
    BaseURL       string
    Headers       *http.Header
    Middlewares   []Middleware
    MaxRetries    int
    RetryStrategy BackoffStrategy
    HTTPClient    *http.Client
    // Encoders/Decoders for JSON, XML, YAML
}
```

**RequestBuilder**
```go
type RequestBuilder struct {
    // Fluent API for building requests
    // Methods: Query(), Header(), JSONBody(), PathParam(), etc.
}
```

**Middleware Interface**
```go
type Middleware func(req *http.Request, next RequestFunc) (*http.Response, error)
```

**Response**
```go
type Response struct {
    RawResponse *http.Response
    BodyBytes   []byte
    // Methods: ScanJSON(), ScanXML(), String(), SaveToFile(), etc.
}
```

## Coding Rules

### Go 1.26 Modern Features

- **Use `maps.Copy()` and `maps.Clone()`** for map operations (see `request.go:78`)
- **Use `slices` package** for slice operations where applicable
- **Leverage Swiss Tables** for map performance (automatic in Go 1.24+)
- **Use `iter.Seq` for custom iterators** (see `response.go:10` for iterator usage)
- **Use `errors.Join()`** for aggregating multiple errors

### Memory Management

- **Buffer Pooling**: Use `GetBuffer()` and `PutBuffer()` from `pool.go` for temporary buffers
- **Pre-allocation**: Pre-allocate maps/slices when size is known (see `request.go:76`)
- **Zero-Copy**: Minimize data copying in hot paths

### Error Handling

- **Sentinel Errors**: Use package-level sentinel errors (e.g., `ErrResponseReadFailed`)
- **Error Wrapping**: Wrap errors with `fmt.Errorf("%w: %w", ...)` for context
- **No Panics**: Return errors instead of panicking; let callers decide how to handle

### API Design

- **Fluent Builders**: All request configuration methods return `*RequestBuilder` for chaining
- **Functional Options**: Use functional options pattern for optional configuration
- **Interface Segregation**: Small, focused interfaces (e.g., `Encoder`, `Decoder`, `Logger`)

## Testing

### Test Structure

- **Co-located Tests**: Tests are in `*_test.go` files alongside source
- **Test Server**: Use `httptest.Server` for HTTP endpoint testing (see `client_test.go:26`)
- **Assertions**: Use `github.com/stretchr/testify/assert` and `github.com/test-go/testify/require`
- **Race Detection**: Enabled by default in `task test`

### Running Tests

```bash
task test                        # Run all tests with race detection
go test -v ./...                 # Verbose test output
go test -run TestClientGet       # Run specific test
go test -bench=. -benchmem ./... # Run benchmarks with memory stats
```

### Test Patterns

- Use `httptest.Server` for mocking HTTP endpoints
- Test both success and error paths
- Verify middleware chain execution order
- Test retry logic with failing endpoints
- Test streaming with large responses

## Dependencies

### Production Dependencies

- **github.com/go-json-experiment/json** - Experimental JSON v2 (faster, more flexible)
- **github.com/goccy/go-yaml** - YAML encoding/decoding
- **github.com/google/go-querystring** - Struct-to-query-string conversion
- **github.com/valyala/bytebufferpool** - High-performance buffer pooling
- **golang.org/x/net** - HTTP/2 support

### Test Dependencies

- **github.com/stretchr/testify** - Assertions and mocking
- **github.com/test-go/testify** - Enhanced testify features

### Dependency Notes

- Using experimental `go-json-experiment/json` for better performance and API
- Buffer pooling is critical for high-throughput scenarios; don't remove without profiling

## Performance

### Optimization Strategies

- **Buffer Pooling**: `pool.go` uses `valyala/bytebufferpool` for response body buffering
- **Pre-allocation**: Maps and slices are pre-allocated when size is predictable
- **HTTP/2**: Enabled via `Config.HTTP2` for connection multiplexing
- **Streaming**: Use streaming callbacks for large responses to avoid memory spikes

### Benchmarking

```bash
go test -bench=. -benchmem ./...           # Run all benchmarks
go test -bench=BenchmarkClient -benchmem   # Run specific benchmark
```

## Agent Skills

This package has access to shared agent skills at `.claude/skills`:

- **agent-md-creating**: Generate CLAUDE.md for Go projects
- **code-simplifying**: Refine and simplify Go code for clarity
- **committing**: Create conventional commits for Go packages
- **dependency-selecting**: Select Go dependencies from vetted libraries
- **go-best-practices**: Google Go coding best practices and style guide
- **linting**: Set up and run golangci-lint v2 for Go projects
- **modernizing**: Go code modernization guide (Go 1.20-1.26 features)
- **ralphy-initializing**: Initialize Ralphy AI coding loop configuration
- **ralphy-todo-creating**: Create Ralphy TODO.yaml task files
- **readme-creating**: Generate README.md for Go libraries
- **releasing**: Guide release process for Go packages
- **testing**: Write Go tests following best practices

Use these skills when working on related tasks (e.g., use `testing` skill when writing tests, `modernizing` when refactoring code).

## Additional Documentation

Detailed documentation is available in the `docs/` directory:

- **docs/client.md** - Client configuration and setup
- **docs/request.md** - Request building and sending
- **docs/response.md** - Response handling and parsing
- **docs/middleware.md** - Middleware system and custom middleware
- **docs/retry.md** - Retry mechanisms and backoff strategies
- **docs/stream.md** - Streaming support for real-time data
- **docs/logging.md** - Logging configuration
