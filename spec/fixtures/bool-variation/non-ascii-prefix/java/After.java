package flags;

import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.ImmutableContext;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean checkoutEnabled(LDClient ldClient, String userKey) {
        /* ü 😀 */ boolean on = client.getBooleanValue("new-checkout", false, new ImmutableContext(userKey)).booleanValue();
        return on;
    }
}