package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import dev.openfeature.sdk.Client;
import dev.openfeature.sdk.OpenFeatureAPI;

public class Checkout {
    interface AuditLogger {
        void log(String message);
    }

    private final Client client = OpenFeatureAPI.getInstance().getClient("checkout");

    public boolean checkoutEnabled(LDClient ldClient, AuditLogger client, String userKey) {
        client.log("checking");
        return ldClient.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
