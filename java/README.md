# Java

Java support is planned for milestone M3. Nothing is implemented yet.

The plan is OpenRewrite recipes that use type attribution to prove a call is
bound to the LaunchDarkly client before rewriting it. The rule-to-Java method
mapping in `spec/rules.yaml` is unverified and must be checked against the real
SDK before any recipe relies on it.

Until M3 this directory contains only this file.
