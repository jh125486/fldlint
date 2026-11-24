package io.github.jh125486.fldlint.planner;

import io.github.jh125486.fldlint.spec.Rule;
import io.github.jh125486.fldlint.spec.RulesGen;

/**
 * Stub planner. Real detection arrives with the Java engine (#10); until then
 * every call site is reported as skipped because client binding cannot be
 * proven, the same contract as go/internal/planner and the JS planner.
 */
public final class Planner {
    /** Skip category reported while no call site can be bound to a client. */
    public static final String CATEGORY_UNBOUND_CLIENT = "unbound-client";

    private Planner() {
    }

    /** Plans one detected LaunchDarkly call, identified by its method name. */
    public static PlanResult plan(String method) {
        Rule rule = RulesGen.lookupJavaMethod(method);
        return PlanResult.skip(CATEGORY_UNBOUND_CLIENT, rule == null ? "" : rule.getId());
    }
}
