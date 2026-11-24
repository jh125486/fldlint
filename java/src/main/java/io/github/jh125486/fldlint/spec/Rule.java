package io.github.jh125486.fldlint.spec;

import java.util.Map;

/** One entry of rules.yaml. Argument indexes are -1 when absent. */
public final class Rule {
    private final String id;
    private final String valueType;
    private final String risk;
    private final int keyArg;
    private final int defaultArg;
    private final int contextArg;
    private final String skip;
    private final String message;
    private final Map<String, Binding> languages;

    public Rule(
            String id,
            String valueType,
            String risk,
            int keyArg,
            int defaultArg,
            int contextArg,
            String skip,
            String message,
            Map<String, Binding> languages) {
        this.id = id;
        this.valueType = valueType;
        this.risk = risk;
        this.keyArg = keyArg;
        this.defaultArg = defaultArg;
        this.contextArg = contextArg;
        this.skip = skip;
        this.message = message;
        this.languages = languages;
    }

    public String getId() {
        return id;
    }

    public String getValueType() {
        return valueType;
    }

    public String getRisk() {
        return risk;
    }

    public int getKeyArg() {
        return keyArg;
    }

    public int getDefaultArg() {
        return defaultArg;
    }

    public int getContextArg() {
        return contextArg;
    }

    /** Rule-level skip category; empty for a rewriting rule. */
    public String getSkip() {
        return skip;
    }

    /** Diagnostic text of a rewritable finding; the only placeholder is {{key}}. */
    public String getMessage() {
        return message;
    }

    /** Bindings by language name ("go", "java", "js"). */
    public Map<String, Binding> getLanguages() {
        return languages;
    }
}
