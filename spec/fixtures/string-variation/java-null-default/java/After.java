package flags;

import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.ImmutableContext;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public String greeting(LDClient ldClient, String userKey) {
        return client.getStringValue("greeting", null, new ImmutableContext(userKey));
    }
}
