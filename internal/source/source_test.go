package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"shireesh.com/gallium/internal/generator"
)

func TestParseGitRef(t *testing.T) {
	cases := []struct {
		ref, url, subdir, version string
	}{
		{"github.com/x/tpl", "github.com/x/tpl", "", ""},
		{"github.com/x/tpl@v1", "github.com/x/tpl", "", "v1"},
		{"github.com/x/tpls//py@v1.2.0", "github.com/x/tpls", "py", "v1.2.0"},
		{"github.com/x/tpls//nested/py", "github.com/x/tpls", "nested/py", ""},
		{"https://github.com/x/tpl.git@v2", "https://github.com/x/tpl.git", "", "v2"},
		{"https://github.com/x/tpls//py", "https://github.com/x/tpls", "py", ""},
		{"git@github.com:x/tpl.git", "git@github.com:x/tpl.git", "", ""},
		{"git@github.com:x/tpl.git@v1", "git@github.com:x/tpl.git", "", "v1"},
	}
	for _, c := range cases {
		url, subdir, version := parseGitRef(c.ref)
		if url != c.url || subdir != c.subdir || version != c.version {
			t.Errorf("parseGitRef(%q) = (%q, %q, %q), want (%q, %q, %q)",
				c.ref, url, subdir, version, c.url, c.subdir, c.version)
		}
	}
}

func TestLooksLikeGitURL(t *testing.T) {
	for _, ref := range []string{"github.com/x/y", "git@github.com:x/y.git", "https://gitlab.com/x/y", "code.example.org/tpl"} {
		if !looksLikeGitURL(ref) {
			t.Errorf("looksLikeGitURL(%q) = false, want true", ref)
		}
	}
	for _, ref := range []string{"python-dev", "private/wordpress", "my.template"} {
		if looksLikeGitURL(ref) {
			t.Errorf("looksLikeGitURL(%q) = true, want false", ref)
		}
	}
}

func TestPickVersion(t *testing.T) {
	tags := []string{"v1.0.0", "v1.2.0", "v1.2.3", "v2.0.0", "release-x", "v2.1.0"}
	cases := []struct {
		requested, want string
	}{
		{"", "v2.1.0"},
		{"v1", "v1.2.3"},
		{"v1.2", "v1.2.3"},
		{"v1.0.0", "v1.0.0"},
		{"release-x", "release-x"},
		{"main", "main"},
	}
	for _, c := range cases {
		got, err := pickVersion(tags, c.requested)
		if err != nil || got != c.want {
			t.Errorf("pickVersion(%q) = (%q, %v), want %q", c.requested, got, err, c.want)
		}
	}

	if got, err := pickVersion(nil, ""); err != nil || got != "" {
		t.Errorf("pickVersion(no tags, \"\") = (%q, %v), want default branch", got, err)
	}
	if _, err := pickVersion(tags, "v9"); err == nil {
		t.Error("pickVersion(v9) should fail when no tag matches")
	}
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "sources:\n  - name: private\n    repo: git@github.com:x/tpls.git\n  - name: local\n    path: ~/templates\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GALLIUM_CONFIG", path)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 2 || cfg.Find("private") == nil || cfg.Find("local") == nil {
		t.Errorf("unexpected config: %+v", cfg)
	}

	t.Setenv("GALLIUM_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg, err = LoadConfig()
	if err != nil || len(cfg.Sources) != 0 {
		t.Errorf("missing config should load empty, got %+v, %v", cfg, err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@test"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initTemplateRepo creates a git repo that is itself a single template, with
// tags v1.0.0 (README says "one") and v1.1.0 (README says "two").
func initTemplateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, ".template"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "description: a test template\ndata:\n  greeting: hello\n"
	if err := os.WriteFile(filepath.Join(dir, ".template", "metadata.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("one {{ .ProjectName }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "v1")
	git(t, dir, "tag", "v1.0.0")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("two {{ .ProjectName }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "v1.1")
	git(t, dir, "tag", "v1.1.0")
	return dir
}

func TestGitSourceResolvesTags(t *testing.T) {
	t.Setenv("GALLIUM_CACHE_DIR", t.TempDir())
	repo := initTemplateRepo(t)
	cfg := &Config{Sources: []SourceConfig{{Name: "private", Repo: repo}}}

	// Latest tag by default.
	res, err := Resolve(nil, cfg, "private")
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "v1.1.0" || res.Template != "." || res.Ref != "private" {
		t.Errorf("resolved = %+v, want version v1.1.0 at repo root", res)
	}
	dst := filepath.Join(t.TempDir(), "out")
	opts := generator.Options{Vars: map[string]string{"ProjectName": "demo"}}
	if err := generator.Generate(res.FS, res.Template, dst, opts); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two demo" {
		t.Errorf("generated README = %q, want %q", data, "two demo")
	}

	// Pinned older tag.
	res, err = Resolve(nil, cfg, "private@v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "v1.0.0" {
		t.Errorf("resolved version = %q, want v1.0.0", res.Version)
	}
	dst = filepath.Join(t.TempDir(), "out2")
	if err := generator.Generate(res.FS, res.Template, dst, opts); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dst, "README.md"))
	if string(data) != "one demo" {
		t.Errorf("generated README = %q, want %q", data, "one demo")
	}
}

func TestLocalPathSourceCollection(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "svc", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "svc", "main.txt"), []byte("{{ .ProjectName }}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Sources: []SourceConfig{{Name: "work", Path: base}}}

	res, err := Resolve(nil, cfg, "work/svc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Template != "svc" || res.Ref != "work/svc" {
		t.Errorf("resolved = %+v", res)
	}

	if _, err := Resolve(nil, cfg, "work"); err == nil {
		t.Error("resolving a collection without a template name should fail")
	}
	if _, err := Resolve(nil, cfg, "work/missing"); err == nil {
		t.Error("resolving a missing template should fail")
	}
}

func TestResolveLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Resolve(nil, &Config{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Template != "." {
		t.Errorf("local dir should resolve at root, got %+v", res)
	}
}

func TestListIncludesAllSources(t *testing.T) {
	t.Setenv("GALLIUM_CACHE_DIR", t.TempDir())
	embedded := os.DirFS(func() string {
		base := t.TempDir()
		os.MkdirAll(filepath.Join(base, "go-basic"), 0o755)
		return base
	}())
	repo := initTemplateRepo(t)
	local := t.TempDir()
	os.MkdirAll(filepath.Join(local, "svc"), 0o755)

	cfg := &Config{Sources: []SourceConfig{
		{Name: "private", Repo: repo},
		{Name: "work", Path: local},
		{Name: "broken", Repo: filepath.Join(local, "nope")},
	}}
	entries, warnings := List(embedded, cfg)

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	for _, want := range []string{"go-basic", "private", "work/svc"} {
		if !names[want] {
			t.Errorf("List missing %q; got %v", want, names)
		}
	}
	if len(warnings) != 1 {
		t.Errorf("want one warning for the broken source, got %v", warnings)
	}
}
