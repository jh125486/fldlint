import com.launchdarkly.sdk.server.interfaces.LDClientInterface;
import com.launchdarkly.sdk.LDContext;

class Ternary {
    Integer f(LDClientInterface ld, dev.openfeature.sdk.Client of, LDContext c, boolean b, Integer other) {
        // int : Integer is a numeric conditional of type int, so a null `other` throws NPE.
        // Without the unboxing call it becomes Integer : Integer (reference conditional),
        // and the null passes through.
        return b ? of.getIntegerValue("n", 0, c).intValue() : other;
    }
}
