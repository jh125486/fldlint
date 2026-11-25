package io.github.jh125486.fldlint;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import org.openrewrite.java.JavaParser;

/**
 * How fixtures are parsed and compiled. Almost every case sees the real LaunchDarkly 7.18.1 and
 * OpenFeature 1.23.0 jars. The v6 {@code LDUser} overload no longer exists in 7.x, so the cases in
 * {@link #V6_CASES} get a stub of the v6 interface instead of the LaunchDarkly jars (a stub and a
 * real jar for the same type must not meet).
 */
final class FixtureParsers {
  /** Case paths (rule/case) that need the v6 stubs. */
  static final List<String> V6_CASES = Collections.singletonList("bool-variation/java-ld-user");

  private static final String[] V6_STUB_PATHS = {
    "stubs/ld-v6/com/launchdarkly/sdk/LDUser.java.txt",
    "stubs/ld-v6/com/launchdarkly/sdk/server/LDClient.java.txt",
    "stubs/ld-v6/com/launchdarkly/sdk/server/interfaces/LDClientInterface.java.txt"
  };

  private FixtureParsers() {}

  static boolean usesV6Stubs(String casePath) {
    return V6_CASES.contains(casePath);
  }

  static JavaParser.Builder<?, ?> forCase(String casePath) {
    if (usesV6Stubs(casePath)) {
      return JavaParser.fromJavaVersion().classpath("sdk").dependsOn(v6Stubs());
    }
    return JavaParser.fromJavaVersion().classpath("launchdarkly-java-server-sdk", "sdk");
  }

  /** The v6 stub sources, as strings. */
  static String[] v6Stubs() {
    String[] sources = new String[V6_STUB_PATHS.length];
    for (int i = 0; i < sources.length; i++) {
      sources[i] = read(V6_STUB_PATHS[i]);
    }
    return sources;
  }

  /** The v6 stub files' names, for javac, paired with their sources. */
  static List<String> v6StubNames() {
    List<String> names = new ArrayList<String>();
    for (String path : V6_STUB_PATHS) {
      names.add(path.substring(path.lastIndexOf('/') + 1).replace(".java.txt", ".java"));
    }
    return names;
  }

  /**
   * The runtime classpath entries, minus the LaunchDarkly jars when the case uses the v6 stubs.
   * Surefire may hand the JVM a manifest-only jar; its real path list is in a system property.
   */
  static List<String> compileClasspath(String casePath) {
    String raw = System.getProperty("surefire.test.class.path");
    if (raw == null || raw.isEmpty()) {
      raw = System.getProperty("java.class.path");
    }
    List<String> entries = new ArrayList<String>();
    for (String entry : raw.split(java.io.File.pathSeparator)) {
      boolean launchDarkly = entry.contains("launchdarkly");
      if (!(usesV6Stubs(casePath) && launchDarkly)) {
        entries.add(entry);
      }
    }
    return Arrays.asList(entries.toArray(new String[0]));
  }

  private static String read(String resource) {
    try (InputStream in = FixtureParsers.class.getClassLoader().getResourceAsStream(resource)) {
      if (in == null) {
        throw new IllegalStateException("missing test resource " + resource);
      }
      java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream();
      byte[] buf = new byte[4096];
      int n;
      while ((n = in.read(buf)) > 0) {
        out.write(buf, 0, n);
      }
      return new String(out.toByteArray(), StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new UncheckedIOException(e);
    }
  }
}
