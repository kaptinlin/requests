# Configuring Logging

This section describes how to activate and tailor logging in the Requests library. While logging is off by default to maintain a clean output, enabling it can provide valuable insights into the HTTP request and response process for troubleshooting and monitoring purposes.

### Enabling Logging

To turn on logging, you must explicitly initialize and set a `Logger` in the client configuration. Here's how to create and use the `DefaultLogger`, which logs to `os.Stderr` by default, and is configured to log errors only:

```go
logger := requests.NewDefaultLogger(os.Stderr, slog.LevelError)
client := requests.Create(&requests.Config{
    Logger: logger,
})
```

Or, for an already instantiated client:

```go
client.SetLogger(requests.NewDefaultLogger(os.Stderr, slog.LevelError))
```

### Adjusting Log Levels

Adjusting the log level is straightforward. After defining your logger, simply set the desired level. This allows you to control the verbosity of the logs based on your requirements.

```go
logger := requests.NewDefaultLogger(os.Stderr, requests.LevelError)
logger.SetLevel(requests.LevelInfo) // Set to Info level to capture more detailed logs

client := requests.Create(&requests.Config{
    Logger: logger,
})
```

The available log levels are:

- `LevelDebug`
- `LevelInfo`
- `LevelWarn`
- `LevelError`

### Implementing a Custom Logger

For more advanced scenarios where you might want to integrate with an existing logging system or format logs differently, implement the `Logger` interface. This requires methods for each level of logging (`Debugf`, `Infof`, `Warnf`, `Errorf`) and a method to set the log level (`SetLevel`).

Here is a simplified example:

```go
type MyLogger struct {
    // Include your custom logging mechanism here
}

func (l *MyLogger) Debugf(format string, v ...any) {
    // Custom debug logging implementation
}

func (l *MyLogger) Infof(format string, v ...any) {
    // Custom info logging implementation
}

func (l *MyLogger) Warnf(format string, v ...any) {
    // Custom warn logging implementation
}

func (l *MyLogger) Errorf(format string, v ...any) {
    // Custom error logging implementation
}

func (l *MyLogger) SetLevel(level requests.Level) {
    // Implement setting the log level in your logger
}

// Usage
myLogger := &MyLogger{}
myLogger.SetLevel(requests.LevelDebug) // Example setting to Debug level

client := requests.Create(&requests.Config{
    Logger: myLogger,
})
```

By implementing the `Logger` interface, you can fully customize how and what you log, making it easy to integrate with any logging framework or system you're already using.
