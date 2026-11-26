package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public int pageSize(LDClient ldClient, String userKey) {
        return ldClient.intVariation("page-size", LDContext.create(userKey), 20);
    }
}
