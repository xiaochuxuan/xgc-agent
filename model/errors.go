// errors.go: Shared errors for the model package.
package model

import "errors"

// ErrNilRequest is returned when a nil request is provided.
var ErrNilRequest = errors.New("nil request")

// ErrMissingAPIKey is returned when the API key is missing.
var ErrMissingAPIKey = errors.New("missing API key")

// ErrUnsupportedProvider is returned when an unsupported provider is specified.
var ErrUnsupportedProvider = errors.New("unsupported provider")
