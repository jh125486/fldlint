// Package ldvalue is a test stub of go-sdk-common v3 ldvalue.
package ldvalue

// Value is a stub JSON value.
type Value struct{}

// Null returns the null value.
func Null() Value { return Value{} }

// JSONString renders the value.
func (v Value) JSONString() string { return "null" }
