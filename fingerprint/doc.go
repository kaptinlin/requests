// Package fingerprint provides optional TLS ClientHello fingerprint profiles for requests.
//
// The package keeps uTLS outside the core requests module. It applies fingerprints
// at the client transport layer through requests.Profile.
package fingerprint
