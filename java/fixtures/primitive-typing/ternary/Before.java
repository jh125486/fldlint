import com.launchdarkly.sdk.server.interfaces.LDClientInterface;
import com.launchdarkly.sdk.LDContext;

class Ternary {
    long f(LDClientInterface ld, dev.openfeature.sdk.Client of, LDContext c, boolean b, long other) {
        // Operand types decide the ternary type; keep int, not Integer.
        return b ? ld.intVariation("n", c, 0) : other;
    }
}
