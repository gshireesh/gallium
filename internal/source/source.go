package source

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"shireesh.com/gallium/internal/generator"
)

// Resolved is a template reference resolved to a concrete filesystem.
type Resolved struct {
	FS       fs.FS
	Template string // template path within FS; "." when FS itself is the template
	Ref      string // canonical reference for stamping, without version
	Version  string // resolved tag for git sources, "" otherwise
}

// Entry describes one available template for listings and the picker.
type Entry struct {
	Name        string // what the user passes to -t
	Description string
	Source      string // "embedded", "<name> (git)", or a local path
}

// isTemplateRoot reports whether dir within fsys is itself a template
// (marked by a .template directory) rather than a collection of templates.
func isTemplateRoot(fsys fs.FS, dir string) bool {
	info, err := fs.Stat(fsys, path.Join(path.Clean(dir), ".template"))
	return err == nil && info.IsDir()
}

// templateDirs lists the top-level directories of a collection.
func templateDirs(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Resolve turns a template reference into a Resolved template. Supported forms:
//
//	python-dev                     embedded template
//	private/wordpress[@v1]         named source from config / template
//	private[@v1]                   named source whose repo root is a template
//	github.com/x/tpl[@v1]          git repository (root is the template)
//	github.com/x/tpls//py[@v1]     subdirectory of a git repository
//	./dir, ../dir, /dir, ~/dir     local directory that is a template
func Resolve(embedded fs.FS, cfg *Config, ref string) (*Resolved, error) {
	if ref == "" {
		return nil, errors.New("empty template reference")
	}

	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") ||
		strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") || ref == "." {
		dir, err := expandPath(ref)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("local template %s: not a directory", ref)
		}
		return &Resolved{FS: os.DirFS(dir), Template: ".", Ref: ref}, nil
	}

	first, rest, _ := strings.Cut(ref, "/")
	name, version, _ := cutVersion(first)
	if src := cfg.Find(name); src != nil {
		tplName := ""
		if rest != "" {
			var restVersion string
			tplName, restVersion, _ = cutVersion(rest)
			if restVersion != "" {
				version = restVersion
			}
		}
		return resolveConfigured(src, tplName, version)
	}

	if looksLikeGitURL(ref) {
		return resolveGitRef(ref)
	}

	if _, err := fs.Stat(embedded, ref); err == nil {
		return &Resolved{FS: embedded, Template: ref, Ref: ref}, nil
	}
	return nil, fmt.Errorf("template %q not found (not embedded, not a configured source, not a git URL)", ref)
}

// cutVersion splits a trailing @version off a name.
func cutVersion(s string) (name, version string, found bool) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

func resolveGitRef(ref string) (*Resolved, error) {
	url, subdir, version := parseGitRef(ref)
	dir, resolvedVersion, err := fetchGit(url, version)
	if err != nil {
		return nil, err
	}

	canonical := url
	tpl := "."
	if subdir != "" {
		tpl = subdir
		canonical = url + "//" + subdir
	}
	fsys := os.DirFS(dir)
	if _, err := fs.Stat(fsys, path.Clean(tpl)); err != nil {
		return nil, fmt.Errorf("%s: no directory %q in repository", url, subdir)
	}
	return &Resolved{FS: fsys, Template: tpl, Ref: canonical, Version: resolvedVersion}, nil
}

func resolveConfigured(src *SourceConfig, tplName, version string) (*Resolved, error) {
	if src.Path != "" {
		dir, err := expandPath(src.Path)
		if err != nil {
			return nil, err
		}
		fsys := os.DirFS(dir)
		if isTemplateRoot(fsys, ".") && tplName == "" {
			return &Resolved{FS: fsys, Template: ".", Ref: src.Name}, nil
		}
		if tplName == "" {
			return nil, fmt.Errorf("source %q is a collection; use -t %s/<template>", src.Name, src.Name)
		}
		if _, err := fs.Stat(fsys, tplName); err != nil {
			return nil, fmt.Errorf("template %q not found in source %q (%s)", tplName, src.Name, dir)
		}
		return &Resolved{FS: fsys, Template: tplName, Ref: src.Name + "/" + tplName}, nil
	}

	dir, resolvedVersion, err := fetchGit(src.Repo, version)
	if err != nil {
		return nil, err
	}
	fsys := os.DirFS(dir)
	if isTemplateRoot(fsys, ".") {
		if tplName != "" {
			return nil, fmt.Errorf("source %q is a single template; use -t %s", src.Name, src.Name)
		}
		return &Resolved{FS: fsys, Template: ".", Ref: src.Name, Version: resolvedVersion}, nil
	}
	if tplName == "" {
		return nil, fmt.Errorf("source %q is a collection; use -t %s/<template>", src.Name, src.Name)
	}
	if _, err := fs.Stat(fsys, tplName); err != nil {
		return nil, fmt.Errorf("template %q not found in source %q (%s)", tplName, src.Name, src.Repo)
	}
	return &Resolved{FS: fsys, Template: tplName, Ref: src.Name + "/" + tplName, Version: resolvedVersion}, nil
}

// List enumerates templates across the embedded set and all configured
// sources. Sources that fail to enumerate (e.g. an unreachable git remote)
// are reported as warnings instead of failing the whole listing.
func List(embedded fs.FS, cfg *Config) (entries []Entry, warnings []string) {
	addCollection := func(fsys fs.FS, prefix, sourceLabel string) error {
		names, err := templateDirs(fsys)
		if err != nil {
			return err
		}
		for _, name := range names {
			entries = append(entries, Entry{
				Name:        prefix + name,
				Description: describe(fsys, name),
				Source:      sourceLabel,
			})
		}
		return nil
	}

	if err := addCollection(embedded, "", "embedded"); err != nil {
		warnings = append(warnings, fmt.Sprintf("embedded templates: %v", err))
	}

	for _, src := range cfg.Sources {
		var fsys fs.FS
		var label string
		if src.Path != "" {
			dir, err := expandPath(src.Path)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("source %s: %v", src.Name, err))
				continue
			}
			fsys, label = os.DirFS(dir), dir
		} else {
			dir, _, err := fetchGit(src.Repo, "")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("source %s: %v", src.Name, err))
				continue
			}
			fsys, label = os.DirFS(dir), src.Name+" (git)"
		}

		if isTemplateRoot(fsys, ".") {
			entries = append(entries, Entry{Name: src.Name, Description: describe(fsys, "."), Source: label})
			continue
		}
		if err := addCollection(fsys, src.Name+"/", label); err != nil {
			warnings = append(warnings, fmt.Sprintf("source %s: %v", src.Name, err))
		}
	}
	return entries, warnings
}

func describe(fsys fs.FS, templateName string) string {
	meta, err := generator.LoadMetadata(fsys, templateName)
	if err != nil {
		return ""
	}
	return meta.Description
}
