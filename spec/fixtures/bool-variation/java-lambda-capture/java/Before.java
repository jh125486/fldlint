package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;
import java.util.function.Supplier;

public class Checkout {
    public Supplier<Boolean> checkoutEnabled(LDClient ldClient, String userKey) {
        Client client = OpenFeatureAPI.getInstance().getClient("checkout");
        client = OpenFeatureAPI.getInstance().getClient("checkout-v2");
        return () -> ldClient.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
