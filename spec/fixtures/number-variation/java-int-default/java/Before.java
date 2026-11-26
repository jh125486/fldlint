package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public double discountRate(LDClient ldClient, String userKey) {
        return ldClient.doubleVariation("discount-rate", LDContext.create(userKey), 1);
    }
}
