package flags;

import static com.launchdarkly.sdk.LDContext.create;

import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean checkoutEnabled(com.launchdarkly.sdk.server.LDClient ldClient, String userKey) {
        return ldClient.boolVariation("new-checkout", create(userKey), false);
    }
}
