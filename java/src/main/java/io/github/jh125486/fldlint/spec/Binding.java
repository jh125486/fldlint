package io.github.jh125486.fldlint.spec;

import java.util.List;
import java.util.Map;

/** The per-language part of a rule (rules.yaml {@code languages.<lang>}). */
public final class Binding {
    private final List<String> methods;
    private final Map<String, Integer> ctxArg;
    private final String template;
    private final List<String> templateArgs;
    private final List<String> requires;
    private final boolean async;

    public Binding(
            List<String> methods,
            Map<String, Integer> ctxArg,
            String template,
            List<String> templateArgs,
            List<String> requires,
            boolean async) {
        this.methods = methods;
        this.ctxArg = ctxArg;
        this.template = template;
        this.templateArgs = templateArgs;
        this.requires = requires;
        this.async = async;
    }

    /** SDK method names this rule is bound to. */
    public List<String> getMethods() {
        return methods;
    }

    /** Index of the leading context argument per method; empty when none. */
    public Map<String, Integer> getCtxArg() {
        return ctxArg;
    }

    /** The rewrite template; empty for report-only rules. */
    public String getTemplate() {
        return template;
    }

    /** Template placeholders in order of appearance (OpenRewrite binds by order, ADR 009 D4). */
    public List<String> getTemplateArgs() {
        return templateArgs;
    }

    /** Skip categories whose proof the rewrite requires. */
    public List<String> getRequires() {
        return requires;
    }

    public boolean isAsync() {
        return async;
    }
}
