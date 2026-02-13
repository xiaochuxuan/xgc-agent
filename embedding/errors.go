package embedding

import "errors"

var (
	ErrDashScopeUnavailable      = errors.New("embedding: DashScope embedding unavailable")
	ErrLocalEmbeddingUnavailable = errors.New("embedding: local embedding unavailable")
	ErrTFIDFEmbeddingUnavailable = errors.New("embedding: TFIDF embedding unavailable")
)
