package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean checkoutEnabled(LDClient ldClient, String userKey) {
        /* ü 😀 */ boolean on = ldClient.boolVariation("new-checkout", LDContext.create(userKey), false);
        return on;
    }
}
