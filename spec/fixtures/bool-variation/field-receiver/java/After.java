package flags;

import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.ImmutableContext;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    static final class Deps {
        LDClient ld;
    }

    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");
    private final Deps deps = new Deps();

    public boolean checkoutEnabled(String userKey) {
        return client.getBooleanValue("new-checkout", false, new ImmutableContext(userKey)).booleanValue();
    }
}