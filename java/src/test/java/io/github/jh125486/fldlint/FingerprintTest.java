package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.github.jh125486.fldlint.spec.SpecJson;
import java.io.IOException;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.List;
import org.junit.jupiter.api.Test;

/** The shared vectors of spec/contracts/fingerprint.md, reproduced by the Java implementation. */
class FingerprintTest {

  private static JsonNode vectors(String name) throws IOException {
    Path file = SpecJson.load().getRoot().resolve("contracts").resolve(name);
    return new ObjectMapper().readTree(file.toFile());
  }

  @Test
  void sharedVectors() throws IOException {
    JsonNode all = vectors("fingerprint-vectors.json");
    assertTrue(all.size() > 20, "vector file is missing or truncated");
    for (JsonNode v : all) {
      String key = v.get("key").isNull() ? null : v.get("key").asText();
      assertEquals(
          v.get("fingerprint").asText(),
          Fingerprint.of(
              v.get("ruleId").asText(), v.get("path").asText(), key, v.get("occ").asInt()),
          v.toString());
    }
  }

  @Test
  void keysThatAreNotWellFormedHashLikeAnAbsentKey() throws IOException {
    JsonNode all = vectors("fingerprint-illformed-vectors.json");
    assertTrue(all.size() > 0, "vector file is missing or truncated");
    for (JsonNode v : all) {
      StringBuilder key = new StringBuilder();
      for (JsonNode unit : v.get("keyUtf16")) {
        key.append((char) unit.asInt());
      }
      assertFalse(Fingerprint.isWellFormed(key.toString()), v.toString());
      assertEquals(
          v.get("fingerprint").asText(),
          Fingerprint.of(
              v.get("ruleId").asText(),
              v.get("path").asText(),
              key.toString(),
              v.get("occ").asInt()),
          v.toString());
    }
  }

  @Test
  void normalizePathFollowsTheContract() {
    assertEquals("a/b", Fingerprint.normalizePath("a//b"));
    assertEquals("a/b", Fingerprint.normalizePath("a/././b"));
    assertEquals("a", Fingerprint.normalizePath("././a"));
    assertEquals("a/b", Fingerprint.normalizePath("a/b/"));
    assertEquals("a/b", Fingerprint.normalizePath(".\\a\\b"));
    assertEquals("Src/a.ts", Fingerprint.normalizePath("Src/a.ts"));
  }

  @Test
  void occurrencesCountPerRuleKindAndKeyInPositionOrder() {
    List<Fingerprint.Positioned> findings =
        Arrays.asList(
            new Fingerprint.Positioned("bool-variation", "k", 10, 5),
            new Fingerprint.Positioned("bool-variation", null, 3, 1),
            new Fingerprint.Positioned("bool-variation", "k", 2, 9),
            new Fingerprint.Positioned("bool-variation", "k", 2, 3),
            new Fingerprint.Positioned("bool-variation", "other", 1, 1),
            new Fingerprint.Positioned("bool-variation", null, 1, 1),
            new Fingerprint.Positioned("bool-variation", "a\ud800", 4, 1));
    assertArrayEquals(new int[] {2, 1, 1, 0, 0, 0, 2}, Fingerprint.assignOccurrences(findings));
  }
}
