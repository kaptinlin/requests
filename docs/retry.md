# Retry Mechanism for HTTP Requests

The retry feature in the Requests library automatically attempts failed HTTP requests again based on certain conditions. This section explains how to set up and adjust these retry strategies for more dependable web interactions.

## Table of Contents
1. [Enhancing Reliability with Retries](#enhancing-reliability-with-retries)
2. [Configuring Retry Strategies](#configuring-retry-strategies)
   - [Applying a Default Backoff Strategy](#applying-a-default-backoff-strategy)
   - [Utilizing a Linear Backoff Strategy](#utilizing-a-linear-backoff-strategy)
   - [Employing an Exponential Backoff Strategy](#employing-an-exponential-backoff-strategy)
3. [Customizing Retry Conditions](#customizing-retry-conditions)
4. [Setting Maximum Retry Attempts](#setting-maximum-retry-attempts)

### Enhancing Reliability with Retries

To set up retries in a fluent, chainable manner, you can configure your client like so:

```go
client := requests.Create(&requests.Config{
    BaseURL: "https://api.example.com",
}).SetRetryStrategy(
    requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second),
).SetRetryIf(
    requests.DefaultRetryIf,
).SetMaxRetries(3)
```

This setup ensures that your client is ready to handle transient failures gracefully.

### Configuring Retry Strategies

#### Applying a Default Backoff Strategy

For consistent delay intervals between retries:

```go
client.SetRetryStrategy(requests.DefaultBackoffStrategy(5 * time.Second))
```

#### Utilizing a Linear Backoff Strategy

To increase delay intervals linearly with each retry attempt:

```go
client.SetRetryStrategy(requests.LinearBackoffStrategy(1 * time.Second))
```

#### Employing an Exponential Backoff Strategy

For exponential delay increases between attempts, with an option to cap the delay:

```go
client.SetRetryStrategy(requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second))
```

#### Adding Jitter to Backoff Strategies

Wrap any backoff strategy with jitter to prevent thundering herd problems. The `JitterBackoffStrategy` applies random ±fraction jitter to each delay:

```go
// Exponential backoff with ±25% jitter
base := requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second)
client.SetRetryStrategy(requests.JitterBackoffStrategy(base, 0.25))

// Linear backoff with ±10% jitter
base := requests.LinearBackoffStrategy(500 * time.Millisecond)
client.SetRetryStrategy(requests.JitterBackoffStrategy(base, 0.1))
```

The `fraction` parameter controls the jitter range: `0.25` means ±25% of the base delay. A fraction of `0` returns the exact base delay.

### Customizing Retry Conditions

Define when retries should be attempted based on response status codes or errors:

```go
client.SetRetryIf(func(req *http.Request, resp *http.Response, err error) bool {
    return resp.StatusCode == http.StatusInternalServerError || err != nil
})
```

### Setting Maximum Retry Attempts

To limit the number of retries, use the `SetMaxRetries` method:

```go
client.SetMaxRetries(3)
```

This method allows you to specify the maximum number of attempts the client should make to execute a request successfully.