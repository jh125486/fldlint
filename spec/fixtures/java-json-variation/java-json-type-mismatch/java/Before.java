package flags;

import com.launchdarkly.sdk.LDContext;
import com.launchdarkly.sdk.server.LDClient;
import com.launchdarkly.sdk.LDValue;

public class Checkout {
    public LDValue banner(LDClient ldClient, String userKey) {
        return ldClient.jsonValueVariation("banner", LDContext.create(userKey), LDValue.ofNull());
    }
}
