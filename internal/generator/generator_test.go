package generator

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemplate lays out a template named "tpl" under a fresh base dir and
// returns an fs.FS rooted at the base. files maps relative paths to contents.
func writeTemplate(t *testing.T, files map[string]string) fs.FS {
	t.Helper()
	base := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(base, "tpl", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return os.DirFS(base)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGenerateRendersVariables(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		"README.md": "# {{ .ProjectName }}\n",
	})
	dst := filepath.Join(t.TempDir(), "out")

	opts := Options{Vars: map[string]string{"ProjectName": "demo"}}
	if err := Generate(fsys, "tpl", dst, opts); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "README.md")); got != "# demo\n" {
		t.Errorf("rendered = %q, want %q", got, "# demo\n")
	}
}

func TestGenerateTemplateAtRoot(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "README.md"), []byte("# {{ .ProjectName }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")

	opts := Options{Vars: map[string]string{"ProjectName": "demo"}}
	if err := Generate(os.DirFS(base), ".", dst, opts); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "README.md")); got != "# demo" {
		t.Errorf("rendered = %q, want %q", got, "# demo")
	}
}

func TestGenerateSkipsTemplateDirButNotSimilarNames(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/metadata.yaml": "data:\n  foo: bar\n",
		"app.template.json":       "{}",
		"nested/.template/x.txt":  "kept: templates can scaffold template repos",
		".DS_Store":               "junk",
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".template")); !os.IsNotExist(err) {
		t.Error("root .template directory was copied into the destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "nested", ".template", "x.txt")); err != nil {
		t.Error("nested .template directory should be copied (it belongs to the content):", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store was copied into the destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "app.template.json")); err != nil {
		t.Error("app.template.json should have been copied:", err)
	}
}

func TestGenerateRunsHooks(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/pre.sh":  "touch pre-ran\n",
		".template/post.sh": "touch post-ran\n",
		"file.txt":          "hi",
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "pre-ran")); err != nil {
		t.Error("pre.sh did not run in the destination:", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "post-ran")); err != nil {
		t.Error("post.sh did not run in the destination:", err)
	}
}

func TestGenerateNoHooksSkipsHooks(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/pre.sh": "touch pre-ran\n",
		"file.txt":         "hi",
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{NoHooks: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "pre-ran")); !os.IsNotExist(err) {
		t.Error("pre.sh ran despite NoHooks")
	}
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Error("file.txt should have been generated:", err)
	}
}

func TestGenerateFailedHookSurfacesError(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/pre.sh": "exit 3\n",
		"file.txt":         "hi",
	})
	dst := filepath.Join(t.TempDir(), "out")

	err := Generate(fsys, "tpl", dst, Options{})
	if err == nil || !strings.Contains(err.Error(), "pre.sh") {
		t.Errorf("want pre.sh failure error, got %v", err)
	}
}

func TestGenerateRefusesOverwriteWithoutForce(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		"file.txt": "new",
	})
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(dst, "file.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Generate(fsys, "tpl", dst, Options{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("want already-exists error, got %v", err)
	}
	if got := readFile(t, filepath.Join(dst, "file.txt")); got != "old" {
		t.Errorf("existing file was modified without --force: %q", got)
	}

	if err := Generate(fsys, "tpl", dst, Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "file.txt")); got != "new" {
		t.Errorf("force overwrite = %q, want %q", got, "new")
	}
}

func TestGenerateRefusedOverwriteSkipsHooks(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/pre.sh": "touch pre-ran\n",
		"file.txt":         "new",
	})
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(dst, "file.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(fsys, "tpl", dst, Options{}); err == nil {
		t.Fatal("want already-exists error, got nil")
	}
	if _, err := os.Stat(filepath.Join(dst, "pre-ran")); !os.IsNotExist(err) {
		t.Error("pre.sh ran despite the refused generation")
	}
}

func TestGenerateRawGlobCopiedVerbatim(t *testing.T) {
	// {{ secrets.TOKEN }} is not valid Go template syntax; without the raw
	// glob this file would fail to parse.
	content := "run: ${{ secrets.TOKEN }}\n"
	fsys := writeTemplate(t, map[string]string{
		".template/metadata.yaml": "raw:\n  - \"*.yml\"\n",
		"ci.yml":                  content,
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "ci.yml")); got != content {
		t.Errorf("raw file = %q, want %q", got, content)
	}
}

func TestGenerateInvalidTemplateSyntaxErrors(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		"ci.yml": "run: ${{ secrets.TOKEN }}\n",
	})
	dst := filepath.Join(t.TempDir(), "out")

	err := Generate(fsys, "tpl", dst, Options{})
	if err == nil || !strings.Contains(err.Error(), "ci.yml") {
		t.Errorf("want parse error naming ci.yml, got %v", err)
	}
}

func TestGenerateBinaryCopiedVerbatim(t *testing.T) {
	content := "PNG\x00\x01{{ not a template }}\x00"
	fsys := writeTemplate(t, map[string]string{
		"logo.png": content,
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dst, "logo.png")); got != content {
		t.Errorf("binary file was altered: %q", got)
	}
}

func TestGenerateShellScriptsExecutable(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		"scripts/run.sh": "echo hi\n",
		"plain.txt":      "hi",
	})
	dst := filepath.Join(t.TempDir(), "out")

	if err := Generate(fsys, "tpl", dst, Options{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dst, "scripts", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("run.sh mode = %v, want executable", info.Mode())
	}
	info, err = os.Stat(filepath.Join(dst, "plain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Errorf("plain.txt mode = %v, want non-executable", info.Mode())
	}
}

func TestGenerateMetadataDefaultsAndPrecedence(t *testing.T) {
	fsys := writeTemplate(t, map[string]string{
		".template/metadata.yaml": "data:\n  Author: default-author\n  License: MIT\n",
		"info.txt":                "{{ .Author }} / {{ .License }} / {{ .ProjectName }}",
	})
	dst := filepath.Join(t.TempDir(), "out")

	vars := map[string]string{"ProjectName": "demo", "Author": "shireesh"}
	if err := Generate(fsys, "tpl", dst, Options{Vars: vars}); err != nil {
		t.Fatal(err)
	}
	want := "shireesh / MIT / demo"
	if got := readFile(t, filepath.Join(dst, "info.txt")); got != want {
		t.Errorf("rendered = %q, want %q", got, want)
	}
	if _, ok := vars["License"]; ok {
		t.Error("Generate mutated the caller's vars map")
	}
}
