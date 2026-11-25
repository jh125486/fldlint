package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.ImmutableContext;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private static final LDContext ANONYMOUS = LDContext.create("anonymous");

    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public LDContext anonymous() {
        return ANONYMOUS;
    }

    public boolean checkoutEnabled(LDClient ldClient, String userKey) {
        return client.getBooleanValue("new-checkout", false, new ImmutableContext(userKey)).booleanValue();
    }
}
