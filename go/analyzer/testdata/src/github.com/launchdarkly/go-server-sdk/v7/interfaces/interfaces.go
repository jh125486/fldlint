// Package interfaces is a test stub of go-server-sdk v7 interfaces. The method
// signatures match the real SDK (interfaces/client_interface.go).
package interfaces

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-sdk-common/v3/ldreason"
)

// LDClientEvaluation is the evaluation part of the client interface.
type LDClientEvaluation interface {
	BoolVariation(key string, context ldcontext.Context, defaultVal bool) (bool, error)
	BoolVariationCtx(ctx context.Context, key string, context ldcontext.Context, defaultVal bool) (bool, error)
	StringVariation(key string, context ldcontext.Context, defaultVal string) (string, error)
}

// LDClientInterface is the full client interface.
type LDClientInterface interface {
	BoolVariation(key string, context ldcontext.Context, defaultVal bool) (bool, error)
	BoolVariationDetail(key string, context ldcontext.Context, defaultVal bool) (bool, ldreason.EvaluationDetail, error)
	Close() error
}

// MigrationOpTracker is a stub tracker.
type MigrationOpTracker interface{}

var _ = ldmigration.Off
