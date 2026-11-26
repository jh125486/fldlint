package flags;

import com.launchdarkly.sdk.LDContext;

public class Checkout {
    /** Not a LaunchDarkly client: the method name alone must not match. */
    static class Toggles {
        boolean boolVariation(String key, LDContext context, boolean fallback) {
            return fallback;
        }
    }

    public boolean checkoutEnabled(Toggles toggles, String userKey) {
        return toggles.boolVariation("new-checkout", LDContext.create(userKey), false);
    }
}
