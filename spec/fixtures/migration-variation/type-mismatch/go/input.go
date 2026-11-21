package flags

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("migration")

var ldClient *ldclient.LDClient

func DBStage(ctx context.Context, userKey string) ldmigration.Stage {
	stage, _, _ := ldClient.MigrationVariation("db-migration", ldcontext.New(userKey), ldmigration.Off)
	if stage == ldmigration.Off {
		stage, _, _ = ldClient.MigrationVariationCtx(ctx, "db-migration", ldcontext.New(userKey), ldmigration.Off)
	}
	return stage
}
