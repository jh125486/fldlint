package io.github.jh125486.fldlint.spec;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.util.Arrays;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class SpecJsonTest {

  @Test
  void explicitRootWithoutSpecJsonIsAHardError(@TempDir Path dir) {
    IllegalStateException e =
        assertThrows(IllegalStateException.class, () -> SpecJson.load(dir.resolve("nope"), true));
    assertTrue(e.getMessage().contains("fldlint.specRoot"), e.getMessage());
  }

  @Test
  void rootWithMalformedSpecJsonIsRejected(@TempDir Path dir) throws IOException {
    Files.createDirectories(dir.resolve("generated"));
    Files.write(
        dir.resolve("generated").resolve("spec.json"),
        "{\"rules\": {}}".getBytes(StandardCharsets.UTF_8));
    assertThrows(IllegalStateException.class, () -> SpecJson.load(dir, true));
  }

  @Test
  void strictDecodingRejectsBadSpecJson(@TempDir Path dir) throws IOException {
    String ok = "{\"rules\": {\"version\": 1, \"rules\": []}, \"cases\": []}";
    String[] bad = {
      ok + "}",
      ok + " garbage",
      "{\"rules\": {\"version\": 1, \"rules\": []}, \"cases\": [], \"cases\": []}",
      "{\"rules\": {\"version\": 1, \"rules\": []}, \"cases\": [], \"extra\": 1}",
    };
    Files.createDirectories(dir.resolve("generated"));
    for (String content : bad) {
      Files.write(
          dir.resolve("generated").resolve("spec.json"), content.getBytes(StandardCharsets.UTF_8));
      assertThrows(IllegalStateException.class, () -> SpecJson.load(dir, true), content);
    }
  }

  private static final ObjectMapper MAPPER = new ObjectMapper();

  private static JsonNode caseWithOutput(boolean output) throws IOException {
    return MAPPER.readTree("{\"case\":{\"expect\":{\"java\":{\"output\":" + output + "}}}}");
  }

  @Test
  void javaFixtureFilesDependOnOutput() throws IOException {
    assertEquals(
        Arrays.asList("Before.java", "After.java"),
        SpecJson.javaFixtureFiles(caseWithOutput(true)));
    assertEquals(Arrays.asList("Before.java"), SpecJson.javaFixtureFiles(caseWithOutput(false)));
  }

  @Test
  void skipFixtureWithoutAfterPassesAndMissingRequiredAfterFails(@TempDir Path dir)
      throws IOException {
    Files.write(dir.resolve("Before.java"), "class A {}".getBytes(StandardCharsets.UTF_8));
    // output:false and no After.java: every required file exists.
    for (String f : SpecJson.javaFixtureFiles(caseWithOutput(false))) {
      assertTrue(Files.isRegularFile(dir.resolve(f)), f);
    }
    // output:true and no After.java: reading the required files (as the harness does) fails.
    assertThrows(
        NoSuchFileException.class,
        () -> {
          for (String f : SpecJson.javaFixtureFiles(caseWithOutput(true))) {
            Files.readAllBytes(dir.resolve(f));
          }
        });
  }

  @Test
  void minimalSpecJsonLoads(@TempDir Path dir) throws IOException {
    Files.createDirectories(dir.resolve("generated"));
    Files.write(
        dir.resolve("generated").resolve("spec.json"),
        "{\"rules\": {\"version\": 1, \"rules\": []}, \"cases\": []}"
            .getBytes(StandardCharsets.UTF_8));
    SpecJson spec = SpecJson.load(dir, true);
    assertEquals(0, spec.cases().size());
    assertEquals(dir.resolve("fixtures").resolve("r").resolve("c"), spec.caseDir("r/c"));
  }
}
