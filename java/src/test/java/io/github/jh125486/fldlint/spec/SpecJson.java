package io.github.jh125486.fldlint.spec;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

/**
 * Test-side reader of spec/generated/spec.json. Java never parses YAML (ADR 011):
 * the JS loader validated every rule and case before it emitted this file, so
 * nothing here validates against the JSON Schema.
 */
public final class SpecJson {
    /** System property that points at a spec root containing generated/spec.json. */
    public static final String SPEC_ROOT_PROPERTY = "fldlint.specRoot";

    /** The spec root used when {@value #SPEC_ROOT_PROPERTY} is not given (relative to java/). */
    public static final String DEFAULT_SPEC_ROOT = "../spec";

    private final Path root;
    private final JsonNode json;

    private SpecJson(Path root, JsonNode json) {
        this.root = root;
        this.json = json;
    }

    /**
     * Loads the spec named by {@value #SPEC_ROOT_PROPERTY}, or the default root.
     * An explicit root without generated/spec.json is a hard error, the same as Go
     * {@code -spec} and the JS {@code --spec-root}.
     */
    public static SpecJson load() throws IOException {
        String explicit = System.getProperty(SPEC_ROOT_PROPERTY);
        if (explicit != null && !explicit.isEmpty()) {
            return load(Paths.get(explicit), true);
        }
        return load(Paths.get(DEFAULT_SPEC_ROOT), false);
    }

    static SpecJson load(Path root, boolean explicit) throws IOException {
        Path file = root.resolve("generated").resolve("spec.json");
        if (!Files.isRegularFile(file)) {
            throw new IllegalStateException((explicit ? "-D" + SPEC_ROOT_PROPERTY + "=" : "default spec root ")
                    + root + " must contain generated/spec.json (not found: " + file + ")");
        }
        JsonNode json = new ObjectMapper().readTree(file.toFile());
        if (json == null || !json.path("cases").isArray() || !json.path("rules").path("rules").isArray()) {
            throw new IllegalStateException(file + ": not a spec.json (need \"rules\" and \"cases\")");
        }
        return new SpecJson(root, json);
    }

    public Path getRoot() {
        return root;
    }

    /** The {@code rules} object: version, skipCategories and rules. */
    public JsonNode rules() {
        return json.get("rules");
    }

    /** The {@code cases} array; each element has {@code path} and {@code case}. */
    public JsonNode cases() {
        return json.get("cases");
    }

    /** The directory holding a case's files, {@code <root>/fixtures/<rule>/<case>}. */
    public Path caseDir(String casePath) {
        return root.resolve("fixtures").resolve(casePath);
    }
}
