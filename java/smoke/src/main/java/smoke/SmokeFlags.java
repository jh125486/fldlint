package smoke;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.LDValue;
import com.launchdarkly.sdk.server.FeatureFlagsState;
import com.launchdarkly.sdk.server.FlagsStateOption;
import com.launchdarkly.sdk.server.LDClient;
import com.launchdarkly.sdk.server.MigrationStage;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

/**
 * Every Java rule once. Rewritten by the migration recipe: the four calls in {@link #rewritten}.
 * Reported only (never rewritten): everything in {@link #reported}. Not a finding: {@link
 * #unrelated}.
 */
public class SmokeFlags {
    private final Client client = OpenFeatureAPI.getInstance().getClient("smoke");

    /** The four rewriting rules: bool, string, number and java-int variation. */
    public String rewritten(LDClient ld, String userKey) {
        boolean on = ld.boolVariation("smoke-bool", LDContext.create(userKey), false);
        String layout = ld.stringVariation("smoke-string", LDContext.create(userKey), "classic");
        double rate = ld.doubleVariation("smoke-number", LDContext.create(userKey), 0.5);
        int size = ld.intVariation("smoke-int", LDContext.create(userKey), 20);
        return on + layout + rate + size;
    }

    /** Skips: default type, dynamic key, detail, json, migration and bulk. */
    public Object reported(LDClient ld, String userKey, String dynamicKey) {
        double wideDefault = ld.doubleVariation("smoke-int-default", LDContext.create(userKey), 1);
        boolean dynamic = ld.boolVariation(dynamicKey, LDContext.create(userKey), false);
        Object detail = ld.boolVariationDetail("smoke-detail", LDContext.create(userKey), false);
        Object json = ld.jsonValueVariation("smoke-json", LDContext.create(userKey), LDValue.ofNull());
        Object migration =
                ld.migrationVariation("smoke-migration", LDContext.create(userKey), MigrationStage.OFF);
        FeatureFlagsState all = ld.allFlagsState(LDContext.create(userKey), FlagsStateOption.WITH_REASONS);
        return wideDefault + "" + dynamic + detail + json + migration + all;
    }

    /** A user class with a method named like an SDK method: not a finding. */
    static class Toggles {
        boolean boolVariation(String key, LDContext context, boolean fallback) {
            return fallback;
        }
    }

    boolean unrelated(Toggles toggles, String userKey) {
        return toggles.boolVariation("smoke-unrelated", LDContext.create(userKey), false);
    }
}
