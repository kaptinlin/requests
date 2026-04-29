// Package http3 provides optional HTTP/3 transport profiles for requests.
//
// The package keeps QUIC dependencies outside the core requests module. Use
// requests.WithProfile(http3.Profile()) when a client should send requests over
// HTTP/3.
package http3
