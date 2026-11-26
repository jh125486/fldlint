package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;

public class Checkout {
    public boolean featureOn(LDClient ldClient, String userKey, Boolean fallback) {
        return ldClient.boolVariation("new-feature", LDContext.create(userKey), fallback);
    }
}
