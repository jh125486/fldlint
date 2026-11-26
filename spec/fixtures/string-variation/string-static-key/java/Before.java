package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public String checkoutLayout(LDClient ldClient, String userKey) {
        return ldClient.stringVariation("checkout-layout", LDContext.create(userKey), "classic");
    }
}
