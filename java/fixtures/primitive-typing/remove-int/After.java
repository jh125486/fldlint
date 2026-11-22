import com.launchdarkly.sdk.server.interfaces.LDClientInterface;
import com.launchdarkly.sdk.LDContext;
import java.util.List;

class RemoveInt {
    void f(LDClientInterface ld, dev.openfeature.sdk.Client of, List<Integer> list) {
        LDContext c = LDContext.create("u");
        // remove(int index): must stay remove(int), not remove(Object).
        list.remove(of.getIntegerValue("idx", 0, c).intValue());
    }
}
