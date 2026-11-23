import com.launchdarkly.sdk.server.interfaces.LDClientInterface;
import com.launchdarkly.sdk.LDContext;

class EqCompare {
    boolean f(LDClientInterface ld, dev.openfeature.sdk.Client of, LDContext c) {
        // int == int: must not become Integer == Integer (reference comparison).
        return of.getIntegerValue("a", 0, c).intValue() == of.getIntegerValue("b", 0, c).intValue();
    }
}
