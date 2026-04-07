package model

import "errors"

// ErrEntryNotFound is returned when a catalog entry cannot be found.
var ErrEntryNotFound = errors.New("entry not found")
