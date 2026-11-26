package flags;

import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.ImmutableContext;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean checkoutEnabled(com.launchdarkly.sdk.server.LDClient ldClient, String userKey) {
        return client.getBooleanValue("new-checkout", false, new ImmutableContext(userKey)).booleanValue();
    }
}
