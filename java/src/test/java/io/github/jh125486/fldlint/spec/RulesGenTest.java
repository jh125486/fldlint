package io.github.jh125486.fldlint.spec;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.junit.jupiter.api.Assumptions;
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

  /** RulesGen is generated from the default spec; other roots legitimately differ. */
  private static void assumeDefaultSpec() {
    Assumptions.assumeTrue(
        SpecJson.isDefaultRoot(), "RulesGen is generated from the default spec root");
  }

  @Test
  void rulesMatchSpecJson() throws IOException {
    assumeDefaultSpec();
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
        Map<String, Integer> ctx = new TreeMap<String, Integer>();
        for (Map.Entry<String, JsonNode> c : e.getValue().path("ctxArg").properties()) {
          ctx.put(c.getKey(), c.getValue().asInt());
        }
        assertEquals(ctx, new TreeMap<String, Integer>(b.getCtxArg()), id);
        assertEquals(placeholders(b.getTemplate()), b.getTemplateArgs(), id);
      }
    }
  }

  /**
   * Independent placeholder extraction for the check above. Edge cases for #10: a placeholder name
   * used twice is listed twice (OpenRewrite binds by order, so the recipe must handle that itself),
   * and a Java template containing a literal double open brace would be read as a mustache
   * placeholder by the generator.
   */
  private static List<String> placeholders(String template) {
    List<String> out = new ArrayList<String>();
    Matcher m = Pattern.compile("\\{\\{([^}]*)\\}\\}|#\\{([^}:]*)(?::[^}]*)?\\}").matcher(template);
    while (m.find()) {
      String name = m.group(1) != null ? m.group(1) : m.group(2);
      if (m.group().startsWith("#{any(")) {
        name = m.group().substring(2, m.group().length() - 1);
      }
      out.add(name);
    }
    return out;
  }

  @Test
  void skipCategoriesMatchSpecJson() throws IOException {
    assumeDefaultSpec();
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
