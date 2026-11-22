// Package interfaces is a test stub of go-server-sdk v7 interfaces. The
// declarations match the real SDK (interfaces/client_interface.go, v7.23.0):
// LDClientInterface embeds LDClientEvaluations and LDClientEvents, and the
// interfaces carry no *Ctx methods (only the concrete LDClient does).
package interfaces

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-sdk-common/v3/ldreason"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	"github.com/launchdarkly/go-server-sdk/v7/interfaces/flagstate"
)

// LDClientEvaluations defines the basic feature flag evaluation methods.
type LDClientEvaluations interface {
	BoolVariation(key string, context ldcontext.Context, defaultVal bool) (bool, error)
	BoolVariationDetail(key string, context ldcontext.Context, defaultVal bool) (bool, ldreason.EvaluationDetail, error)
	IntVariation(key string, context ldcontext.Context, defaultVal int) (int, error)
	Float64Variation(key string, context ldcontext.Context, defaultVal float64) (float64, error)
	StringVariation(key string, context ldcontext.Context, defaultVal string) (string, error)
	JSONVariation(key string, context ldcontext.Context, defaultVal ldvalue.Value) (ldvalue.Value, error)
	MigrationVariation(key string, context ldcontext.Context, defaultStage ldmigration.Stage) (ldmigration.Stage, LDMigrationOpTracker, error)
	AllFlagsState(context ldcontext.Context, options ...flagstate.Option) flagstate.AllFlags
}

// LDClientEvents defines the event methods implemented by LDClient.
type LDClientEvents interface {
	Identify(context ldcontext.Context) error
	TrackEvent(eventName string, context ldcontext.Context) error
}

// LDClientInterface defines the full client interface.
type LDClientInterface interface {
	LDClientEvaluations
	LDClientEvents
	Close() error
}

// LDMigrationOpTracker is a stub tracker.
type LDMigrationOpTracker interface{}
