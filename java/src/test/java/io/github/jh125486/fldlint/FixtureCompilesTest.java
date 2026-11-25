package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.JsonNode;
import io.github.jh125486.fldlint.spec.SpecJson;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import javax.tools.Diagnostic;
import javax.tools.DiagnosticCollector;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

/**
 * Every Java fixture must be real code: javac compiles each {@code Before.java} and {@code
 * After.java} against the SDK jars (the v6 stubs for the LDUser case). OpenRewrite's type
 * attribution tolerates code javac rejects, so this is the check that the fixtures are valid.
 */
class FixtureCompilesTest {

  @TestFactory
  List<DynamicTest> fixturesCompile() throws IOException {
    SpecJson spec = SpecJson.load();
    List<DynamicTest> tests = new ArrayList<DynamicTest>();
    for (JsonNode testCase : spec.cases()) {
      if (!testCase.path("case").path("expect").has("java")) {
        continue;
      }
      final String path = testCase.get("path").asText();
      final Path dir = spec.caseDir(path).resolve("java");
      for (final String file : SpecJson.javaFixtureFiles(testCase)) {
        tests.add(
            DynamicTest.dynamicTest(
                path + "/" + file + " compiles", () -> compile(path, dir.resolve(file))));
      }
    }
    return tests;
  }

  private static void compile(String casePath, Path source) throws IOException {
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    Path work = Files.createTempDirectory("fldlint-fixture");
    try {
      // The public class is Checkout in every fixture, so the file must be named after it.
      Path file = work.resolve("Checkout.java");
      Files.write(file, Files.readAllBytes(source));
      List<Path> units = new ArrayList<Path>();
      units.add(file);
      if (FixtureParsers.usesV6Stubs(casePath)) {
        String[] stubs = FixtureParsers.v6Stubs();
        List<String> names = FixtureParsers.v6StubNames();
        for (int i = 0; i < stubs.length; i++) {
          Path stub = work.resolve("stub" + i).resolve(names.get(i));
          Files.createDirectories(stub.getParent());
          Files.write(stub, stubs[i].getBytes(StandardCharsets.UTF_8));
          units.add(stub);
        }
      }
      DiagnosticCollector<JavaFileObject> diagnostics = new DiagnosticCollector<JavaFileObject>();
      try (StandardJavaFileManager files =
          compiler.getStandardFileManager(diagnostics, null, StandardCharsets.UTF_8)) {
        List<String> options =
            Arrays.asList(
                "-proc:none",
                "-Xlint:none",
                "-d",
                work.resolve("out").toString(),
                "-classpath",
                String.join(java.io.File.pathSeparator, FixtureParsers.compileClasspath(casePath)));
        Files.createDirectories(work.resolve("out"));
        List<java.io.File> asFiles = new ArrayList<java.io.File>();
        for (Path unit : units) {
          asFiles.add(unit.toFile());
        }
        boolean ok =
            compiler
                .getTask(
                    null,
                    files,
                    diagnostics,
                    options,
                    null,
                    files.getJavaFileObjectsFromFiles(asFiles))
                .call();
        StringBuilder errors = new StringBuilder();
        for (Diagnostic<? extends JavaFileObject> d : diagnostics.getDiagnostics()) {
          if (d.getKind() == Diagnostic.Kind.ERROR) {
            errors.append(d).append('\n');
          }
        }
        assertTrue(ok, source + " does not compile:\n" + errors);
      }
    } finally {
      deleteRecursively(work);
    }
  }

  private static void deleteRecursively(Path root) throws IOException {
    try (java.util.stream.Stream<Path> walk = Files.walk(root)) {
      List<Path> all = new ArrayList<Path>();
      walk.forEach(all::add);
      for (int i = all.size() - 1; i >= 0; i--) {
        Files.delete(all.get(i));
      }
    }
  }
}
