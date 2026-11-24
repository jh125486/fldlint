package io.github.jh125486.fldlint.spec;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

/** Cross-checks the committed RulesGen against the spec.json the JS loader emitted. */
class RulesGenTest {

    private static List<String> strings(JsonNode array) {
        List<String> out = new ArrayList<String>();
        for (JsonNode n : array) {
            out.add(n.asText());
        }
        return out;
    }

    @Test
    void rulesMatchSpecJson() throws IOException {
        JsonNode rules = SpecJson.load().rules().get("rules");
        assertEquals(rules.size(), RulesGen.RULES.size());
        for (int i = 0; i < rules.size(); i++) {
            JsonNode want = rules.get(i);
            Rule got = RulesGen.RULES.get(i);
            String id = want.get("id").asText();
            assertEquals(id, got.getId());
            assertEquals(want.path("valueType").asText(""), got.getValueType(), id);
            assertEquals(want.path("risk").asText(""), got.getRisk(), id);
            assertEquals(want.path("skip").asText(""), got.getSkip(), id);
            assertEquals(want.path("message").asText(""), got.getMessage(), id);
            assertEquals(want.path("keyArg").asInt(-1), got.getKeyArg(), id);
            assertEquals(want.path("defaultArg").asInt(-1), got.getDefaultArg(), id);
            assertEquals(want.path("contextArg").asInt(-1), got.getContextArg(), id);
            JsonNode langs = want.path("languages");
            assertEquals(langs.size(), got.getLanguages().size(), id);
            for (Map.Entry<String, JsonNode> e : langs.properties()) {
                Binding b = got.getLanguages().get(e.getKey());
                assertNotNull(b, id + "/" + e.getKey());
                assertEquals(strings(e.getValue().get("methods")), b.getMethods(), id);
                assertEquals(e.getValue().path("template").asText(""), b.getTemplate(), id);
                assertEquals(strings(e.getValue().path("requires")), b.getRequires(), id);
                assertEquals(e.getValue().path("async").asBoolean(false), b.isAsync(), id);
                assertEquals(e.getValue().path("ctxArg").size(), b.getCtxArg().size(), id);
            }
        }
    }

    @Test
    void skipCategoriesMatchSpecJson() throws IOException {
        JsonNode cats = SpecJson.load().rules().get("skipCategories");
        assertEquals(cats.size(), RulesGen.SKIP_CATEGORIES.size());
        for (Map.Entry<String, JsonNode> e : cats.properties()) {
            SkipCategory got = RulesGen.SKIP_CATEGORIES.get(e.getKey());
            assertNotNull(got, e.getKey());
            assertEquals(e.getValue().get("severity").asText(), got.getSeverity());
            assertEquals(e.getValue().get("message").asText(), got.getMessage());
        }
    }

    @Test
    void javaTemplateArgsAreInTemplateOrder() {
        Rule rule = RulesGen.lookupJavaMethod("boolVariation");
        assertNotNull(rule);
        assertEquals(
                java.util.Arrays.asList("any(dev.openfeature.sdk.Client)", "key", "default", "context"),
                rule.getLanguages().get("java").getTemplateArgs());
        assertNull(RulesGen.lookupJavaMethod("noSuchMethod"));
    }
}
