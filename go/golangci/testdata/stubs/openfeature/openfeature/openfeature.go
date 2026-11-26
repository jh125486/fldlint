// Package openfeature is a test stub of open-feature go-sdk. The method
// signatures match the real SDK (openfeature/client.go, interfaces.go).
package openfeature

import "context"

// EvaluationContext is a stub evaluation context.
type EvaluationContext struct{}

// NewEvaluationContext creates a context.
func NewEvaluationContext(targetingKey string, attributes map[string]any) EvaluationContext {
	return EvaluationContext{}
}

// Option is an evaluation option.
type Option func()

// Client is the stub client.
type Client struct{}

// NewClient creates a client for a domain.
func NewClient(domain string) *Client { return &Client{} }

// BooleanValue evaluates a boolean flag.
func (c *Client) BooleanValue(ctx context.Context, flag string, defaultValue bool, evalCtx EvaluationContext, options ...Option) (bool, error) {
	return defaultValue, nil
}

// StringValue evaluates a string flag.
func (c *Client) StringValue(ctx context.Context, flag string, defaultValue string, evalCtx EvaluationContext, options ...Option) (string, error) {
	return defaultValue, nil
}

// FloatValue evaluates a float flag.
func (c *Client) FloatValue(ctx context.Context, flag string, defaultValue float64, evalCtx EvaluationContext, options ...Option) (float64, error) {
	return defaultValue, nil
}

// IClient is the client interface.
type IClient interface {
	BooleanValue(ctx context.Context, flag string, defaultValue bool, evalCtx EvaluationContext, options ...Option) (bool, error)
	StringValue(ctx context.Context, flag string, defaultValue string, evalCtx EvaluationContext, options ...Option) (string, error)
	FloatValue(ctx context.Context, flag string, defaultValue float64, evalCtx EvaluationContext, options ...Option) (float64, error)
}

var _ IClient = (*Client)(nil)
