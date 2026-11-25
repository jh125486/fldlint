package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client checkoutClient = OpenFeatureAPI.getInstance().getClient("checkout");
    private final Client billingClient = OpenFeatureAPI.getInstance().getClient("billing");

    public boolean checkoutEnabled(LDClient ldClient, String userKey) {
        return ldClient.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
