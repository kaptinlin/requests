# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Go HTTP client library that provides a simplified interface for making HTTP requests. The library is designed as a fluent/builder pattern alternative to Go's standard `net/http` package, offering features like middleware, retry mechanisms, streaming, and various encoding formats.

## Development Commands

### Building and Testing
- `make all` - Run both linting and tests
- `make test` - Run all tests with race detection: `go test -race ./...`
- `make lint` - Run golangci-lint and mod tidy checks
- `make clean` - Clean up binaries in ./bin directory

### Individual Commands
- `go test ./...` - Run tests without race detection
- `go test -v ./...` - Run tests with verbose output
- `go test ./client_test.go` - Run specific test file
- `golangci-lint run` - Run linter (requires installation via make)

## Core Architecture

### Client (`client.go`)
The main `Client` struct serves as the entry point for all HTTP operations. It holds:
- Base URL and default headers/cookies
- HTTP client configuration (timeouts, TLS, proxy)
- Middleware stack for request/response processing
- Encoders/decoders for JSON, XML, YAML
- Retry configuration and authentication methods

### RequestBuilder Pattern (`request.go`)
All HTTP requests use a builder pattern starting from client methods:
```go
client.Get("/path").Query("key", "value").JSONBody(data).Send(ctx)
```

### Response Handling (`response.go`)
The `Response` struct wraps `http.Response` and provides:
- Automatic body reading and buffering
- Streaming support with callbacks
- Content type-aware parsing methods (JSON, XML, YAML)
- File download capabilities

### Core Components
- **Middleware** (`middleware.go`, `middlewares/`): Chain of request/response processors
- **Authentication** (`auth.go`): Support for various auth methods
- **Retry Logic** (`retry.go`): Configurable backoff strategies
- **Streaming** (`stream.go`): Real-time data processing
- **Encoding/Decoding**: JSON (`json.go`), XML (`xml.go`), YAML (`yaml.go`), Forms (`form.go`)

### Architecture Patterns
- **Fluent Builder**: All request configuration uses method chaining
- **Middleware Stack**: Extensible request/response processing pipeline
- **Strategy Pattern**: Pluggable retry strategies, auth methods, encoders
- **Pool Pattern** (`pool.go`): Buffer pooling for memory efficiency
- **Callback Pattern**: Streaming operations use callback functions

## Key Features
- HTTP/2 support with automatic transport configuration
- Built-in retry mechanisms with exponential backoff
- Middleware system for cross-cutting concerns (caching, logging, headers)
- Multiple content-type support (JSON, XML, YAML, forms, multipart)
- Streaming support for real-time data processing
- Certificate and proxy configuration
- Cookie jar management
- Path parameter substitution

## Testing
Tests are co-located with source files using the `_test.go` suffix. The project uses:
- `github.com/stretchr/testify` for assertions and mocking
- Race detection enabled by default in make targets
- Comprehensive test coverage for all major components