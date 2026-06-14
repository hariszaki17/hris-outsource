package domain

import "errors"

// ErrNotFound is the repository-layer sentinel for a missing row. Services
// translate it to the appropriate apperr (e.g. NOT_FOUND, or a domain-specific
// auth error) — repositories never speak HTTP.
var ErrNotFound = errors.New("not found")
