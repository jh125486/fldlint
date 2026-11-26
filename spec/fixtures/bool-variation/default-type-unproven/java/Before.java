package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean featureOn(LDClient ldClient, String userKey, Boolean fallback) {
        return ldClient.boolVariation("new-feature", LDContext.create(userKey), fallback);
    }
}
