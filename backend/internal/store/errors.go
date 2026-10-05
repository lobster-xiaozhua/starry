package store

import "errors"

var (
	ErrConflict = errors.New("resource already exists")
	ErrNotFound = errors.New("resource not found")
)
