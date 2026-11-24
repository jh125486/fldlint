package io.github.jh125486.fldlint.spec;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
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
        Files.write(dir.resolve("generated").resolve("spec.json"), "{\"rules\": {}}".getBytes("UTF-8"));
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
            Files.write(dir.resolve("generated").resolve("spec.json"), content.getBytes("UTF-8"));
            assertThrows(IllegalStateException.class, () -> SpecJson.load(dir, true), content);
        }
    }

    @Test
    void minimalSpecJsonLoads(@TempDir Path dir) throws IOException {
        Files.createDirectories(dir.resolve("generated"));
        Files.write(
                dir.resolve("generated").resolve("spec.json"),
                "{\"rules\": {\"version\": 1, \"rules\": []}, \"cases\": []}".getBytes("UTF-8"));
        SpecJson spec = SpecJson.load(dir, true);
        assertEquals(0, spec.cases().size());
        assertEquals(dir.resolve("fixtures").resolve("r").resolve("c"), spec.caseDir("r/c"));
    }
}
