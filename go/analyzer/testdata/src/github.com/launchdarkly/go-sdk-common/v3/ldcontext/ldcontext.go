// Package ldcontext is a test stub of go-sdk-common v3 ldcontext.
package ldcontext

// Context is a stub evaluation context.
type Context struct{}

// Builder is a stub context builder.
type Builder struct{}

// New creates a single-kind user context.
func New(key string) Context { return Context{} }

// NewWithKind creates a context of a kind.
func NewWithKind(kind, key string) Context { return Context{} }

// NewMulti creates a multi-kind context.
func NewMulti(contexts ...Context) Context { return Context{} }

// NewBuilder starts a builder.
func NewBuilder(key string) *Builder { return &Builder{} }

// Kind sets the kind.
func (b *Builder) Kind(kind string) *Builder { return b }

// Build builds the context.
func (b *Builder) Build() Context { return Context{} }
