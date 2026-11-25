package io.github.jh125486.fldlint;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** The Java implementation of spec/contracts/fingerprint.md. */
public final class Fingerprint {
  private static final char[] HEX = "0123456789abcdef".toCharArray();

  private Fingerprint() {}

  /**
   * Normalizes a path: {@code \} to {@code /}, collapses runs of {@code /}, strips leading {@code
   * ./} (repeatedly) and interior {@code /./}, strips trailing {@code /}. No case folding or
   * Unicode normalization.
   */
  public static String normalizePath(String path) {
    String s = path.replace('\\', '/');
    while (s.contains("//")) {
      s = s.replace("//", "/");
    }
    while (s.startsWith("./")) {
      s = s.substring(2);
    }
    while (s.contains("/./")) {
      s = s.replace("/./", "/");
    }
    int end = s.length();
    while (end > 0 && s.charAt(end - 1) == '/') {
      end--;
    }
    return s.substring(0, end);
  }

  /** True when the string has no unpaired surrogate (JS {@code isWellFormed}). */
  public static boolean isWellFormed(String s) {
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      if (Character.isHighSurrogate(c)) {
        if (i + 1 >= s.length() || !Character.isLowSurrogate(s.charAt(i + 1))) {
          return false;
        }
        i++;
      } else if (Character.isLowSurrogate(c)) {
        return false;
      }
    }
    return true;
  }

  /**
   * The key as a fingerprint input: the string when static and well formed, else null (a key that
   * is not well formed counts as dynamic).
   */
  public static String staticKey(String key) {
    return key != null && isWellFormed(key) ? key : null;
  }

  /**
   * The 32 lowercase hex characters of SHA-256 over {@code ruleId NUL path NUL kind NUL key NUL
   * occ}, in UTF-8. {@code key} is the decoded value of a static key, or null.
   */
  public static String of(String ruleId, String path, String key, int occ) {
    String k = staticKey(key);
    String preimage =
        ruleId
            + '\0'
            + normalizePath(path)
            + '\0'
            + (k == null ? "-" : "s")
            + '\0'
            + (k == null ? "" : k)
            + '\0'
            + occ;
    MessageDigest digest;
    try {
      digest = MessageDigest.getInstance("SHA-256");
    } catch (NoSuchAlgorithmException e) {
      throw new IllegalStateException("SHA-256 is required of every JRE", e);
    }
    byte[] hash = digest.digest(preimage.getBytes(StandardCharsets.UTF_8));
    char[] out = new char[32];
    for (int i = 0; i < 16; i++) {
      out[2 * i] = HEX[(hash[i] >> 4) & 0xF];
      out[2 * i + 1] = HEX[hash[i] & 0xF];
    }
    return new String(out);
  }

  /** One finding's position and identity inputs, for {@link #assignOccurrences(List)}. */
  public static final class Positioned {
    private final String ruleId;
    private final String key;
    private final int line;
    private final int col;

    public Positioned(String ruleId, String key, int line, int col) {
      this.ruleId = ruleId;
      this.key = key;
      this.line = line;
      this.col = col;
    }
  }

  /**
   * The 0-based ordinal of each finding of ONE file among findings with the same (ruleId, kind,
   * key), ordered by (line, col). The result has the input's order.
   */
  public static int[] assignOccurrences(List<Positioned> findings) {
    Map<String, List<Integer>> groups = new HashMap<String, List<Integer>>();
    for (int i = 0; i < findings.size(); i++) {
      Positioned f = findings.get(i);
      String k = staticKey(f.key);
      String id = f.ruleId + '\0' + (k == null ? "-" : "s") + '\0' + (k == null ? "" : k);
      List<Integer> group = groups.get(id);
      if (group == null) {
        group = new ArrayList<Integer>();
        groups.put(id, group);
      }
      group.add(i);
    }
    int[] occ = new int[findings.size()];
    final List<Positioned> all = findings;
    for (List<Integer> group : groups.values()) {
      Collections.sort(
          group,
          new Comparator<Integer>() {
            @Override
            public int compare(Integer a, Integer b) {
              Positioned x = all.get(a);
              Positioned y = all.get(b);
              if (x.line != y.line) {
                return Integer.compare(x.line, y.line);
              }
              if (x.col != y.col) {
                return Integer.compare(x.col, y.col);
              }
              return Integer.compare(a, b);
            }
          });
      for (int rank = 0; rank < group.size(); rank++) {
        occ[group.get(rank)] = rank;
      }
    }
    return occ;
  }
}
