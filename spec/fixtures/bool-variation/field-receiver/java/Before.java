package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    static final class Deps {
        LDClient ld;
    }

    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");
    private final Deps deps = new Deps();

    public boolean checkoutEnabled(String userKey) {
        return this.deps.ld.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
