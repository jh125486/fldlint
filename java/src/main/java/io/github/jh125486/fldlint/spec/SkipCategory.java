package io.github.jh125486.fldlint.spec;

/** A report-only category from spec/rules.yaml skipCategories. */
public final class SkipCategory {
    private final String severity;
    private final String message;

    public SkipCategory(String severity, String message) {
        this.severity = severity;
        this.message = message;
    }

    public String getSeverity() {
        return severity;
    }

    public String getMessage() {
        return message;
    }
}
