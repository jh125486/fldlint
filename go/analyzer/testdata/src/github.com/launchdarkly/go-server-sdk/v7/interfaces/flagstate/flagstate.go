// Package flagstate is a test stub of go-server-sdk v7 interfaces/flagstate.
package flagstate

// Option is a stub option.
type Option interface{}

// AllFlags is a stub snapshot.
type AllFlags struct{}

// ToValuesMap returns the values.
func (a AllFlags) ToValuesMap() map[string]any { return nil }
