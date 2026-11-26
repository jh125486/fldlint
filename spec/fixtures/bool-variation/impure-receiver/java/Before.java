package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    interface Clients {
        LDClient current();
    }

    public boolean checkoutEnabled(Clients clients, String userKey) {
        return clients.current().boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
