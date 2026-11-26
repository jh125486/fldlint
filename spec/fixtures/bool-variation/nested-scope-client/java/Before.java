package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    public String billing() {
        Client billingClient = OpenFeatureAPI.getInstance().getClient("billing");
        return billingClient.getMetadata().getName();
    }

    public boolean checkout(LDClient ldClient, String userKey) {
        return ldClient.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
