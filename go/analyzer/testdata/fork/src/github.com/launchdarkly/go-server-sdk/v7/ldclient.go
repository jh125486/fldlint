// Package ldclient is a test stub of go-server-sdk v7. The method signatures
// match the real SDK (ldclient.go).
package ldclient

import (
	"context"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-sdk-common/v3/ldreason"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	"github.com/launchdarkly/go-server-sdk/v7/interfaces"
	"github.com/launchdarkly/go-server-sdk/v7/interfaces/flagstate"
)

// LDClient is the stub client.
type LDClient struct{}

// MakeClient creates a stub client.
func MakeClient(sdkKey string, waitFor time.Duration) (*LDClient, error) { return &LDClient{}, nil }

// Close closes the client.
func (client *LDClient) Close() error { return nil }

// BoolVariation evaluates a boolean flag.
func (client *LDClient) BoolVariation(defaultVal bool, key string, context ldcontext.Context) (bool, error) {
	return defaultVal, nil
}

// BoolVariationCtx evaluates a boolean flag with a Go context.
func (client *LDClient) BoolVariationCtx(ctx string, key string, context ldcontext.Context, defaultVal bool) (bool, error) {
	return defaultVal, nil
}

// BoolVariationDetail evaluates a boolean flag with detail.
func (client *LDClient) BoolVariationDetail(key string, context ldcontext.Context, defaultVal bool) (bool, ldreason.EvaluationDetail, error) {
	return defaultVal, ldreason.EvaluationDetail{}, nil
}

// StringVariation evaluates a string flag.
func (client *LDClient) StringVariation(key string, context ldcontext.Context, defaultVal string) string {
	return defaultVal
}

// Float64Variation evaluates a number flag.
func (client *LDClient) Float64Variation(key string, context ldcontext.Context, defaultVal float64, extra int) (float64, error) {
	return defaultVal, nil
}

// IntVariation evaluates an int flag.
func (client *LDClient) IntVariation(key string, context ldcontext.Context, defaultVal int) (int, error) {
	return defaultVal, nil
}

// JSONVariation evaluates a JSON flag.
func (client *LDClient) JSONVariation(key string, context ldcontext.Context, defaultVal ldvalue.Value) (ldvalue.Value, error) {
	return defaultVal, nil
}

// MigrationVariation evaluates a migration flag.
func (client *LDClient) MigrationVariation(key string, context ldcontext.Context, defaultStage ldmigration.Stage) (ldmigration.Stage, interfaces.LDMigrationOpTracker, error) {
	return defaultStage, nil, nil
}

// MigrationVariationCtx evaluates a migration flag with a Go context.
func (client *LDClient) MigrationVariationCtx(ctx context.Context, key string, context ldcontext.Context, defaultStage ldmigration.Stage) (ldmigration.Stage, interfaces.LDMigrationOpTracker, error) {
	return defaultStage, nil, nil
}

// AllFlagsState returns every flag.
func (client *LDClient) AllFlagsState(context ldcontext.Context, options ...flagstate.Option) flagstate.AllFlags {
	return flagstate.AllFlags{}
}

// Identify identifies a context.
func (client *LDClient) Identify(context ldcontext.Context) error { return nil }

// TrackEvent tracks an event.
func (client *LDClient) TrackEvent(eventName string, context ldcontext.Context) error { return nil }

var _ interfaces.LDMigrationOpTracker
