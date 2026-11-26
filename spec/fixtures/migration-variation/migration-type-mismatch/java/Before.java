package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import com.launchdarkly.sdk.server.MigrationStage;
import com.launchdarkly.sdk.server.MigrationVariation;

public class Checkout {
    public MigrationStage stage(LDClient ldClient, String userKey) {
        MigrationVariation migration = ldClient.migrationVariation("migrate-db", LDContext.create(userKey), MigrationStage.OFF);
        return migration.getStage();
    }
}
