package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import com.launchdarkly.sdk.server.FeatureFlagsState;
import com.launchdarkly.sdk.server.FlagsStateOption;

public class Checkout {
    public FeatureFlagsState snapshot(LDClient ldClient, String userKey) {
        return ldClient.allFlagsState(LDContext.create(userKey), FlagsStateOption.CLIENT_SIDE_ONLY, FlagsStateOption.WITH_REASONS);
    }
}
