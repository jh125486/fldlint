package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;

public class Checkout {
    public boolean checkoutEnabled(LDClient ldClient, String userKey) {
        return ldClient.boolVariationDetail("new-checkout", LDContext.create(userKey), false).getValue();
    }
}
