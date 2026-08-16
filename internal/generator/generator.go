package generator

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Metadata is the parsed .template/metadata.yaml of a template.
type Metadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// Data holds default variable values; caller-supplied vars take precedence.
	Data map[string]string `yaml:"data"`
	// Raw lists glob patterns (matched against the slash-separated relative
	// path and the base name) for files that must be copied verbatim instead
	// of rendered as Go templates.
	Raw []string `yaml:"raw"`
}

// Options controls a Generate run.
type Options struct {
	Vars    map[string]string
	Force   bool
	NoHooks bool
}

// junkNames are OS metadata files never copied into generated projects.
var junkNames = map[string]bool{".DS_Store": true, "Thumbs.db": true}

// LoadMetadata reads .template/metadata.yaml for templateName within fsys.
// It returns an error wrapping fs.ErrNotExist when the template has no metadata.
func LoadMetadata(fsys fs.FS, templateName string) (*Metadata, error) {
	data, err := fs.ReadFile(fsys, path.Join(path.Clean(templateName), ".template", "metadata.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta Metadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}
	return &meta, nil
}

// Generate renders the template at templateName within fsys into projectName.
// fsys may be any fs.FS: embedded templates, os.DirFS over a local directory,
// or a cloned repository. templateName may be "." when fsys itself is the
// template root.
func Generate(fsys fs.FS, templateName, projectName string, opts Options) error {
	sub, err := fs.Sub(fsys, path.Clean(templateName))
	if err != nil {
		return fmt.Errorf("template %q: %w", templateName, err)
	}
	dst := filepath.Clean(projectName)

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	meta := &Metadata{}
	if m, err := LoadMetadata(sub, "."); err == nil {
		meta = m
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	merged := make(map[string]string, len(opts.Vars)+len(meta.Data))
	maps.Copy(merged, meta.Data)
	maps.Copy(merged, opts.Vars)

	if !opts.Force {
		if err := checkConflicts(sub, dst); err != nil {
			return err
		}
	}

	if !opts.NoHooks {
		if err := runHook(sub, "pre.sh", dst); err != nil {
			return err
		}
	}

	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Only the template's own .template dir is gallium metadata; a
			// nested one belongs to the generated content (e.g. a template
			// that scaffolds a template repo).
			if p == ".template" {
				return fs.SkipDir
			}
			if p == "." {
				return nil
			}
			return os.MkdirAll(filepath.Join(dst, filepath.FromSlash(p)), 0o755)
		}
		if junkNames[d.Name()] {
			return nil
		}

		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}

		out := data
		if !matchesRaw(p, meta.Raw) && !isBinary(data) {
			tpl, err := template.New(p).Parse(string(data))
			if err != nil {
				return fmt.Errorf("failed to parse template file %s: %w", p, err)
			}
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, merged); err != nil {
				return fmt.Errorf("failed to render template file %s: %w", p, err)
			}
			out = buf.Bytes()
		}

		target := filepath.Join(dst, filepath.FromSlash(p))
		return os.WriteFile(target, out, outputMode(p, d))
	})
	if err != nil {
		return err
	}

	if !opts.NoHooks {
		return runHook(sub, "post.sh", dst)
	}
	return nil
}

// checkConflicts runs before any hook or write so a refused generation leaves
// the destination untouched.
func checkConflicts(sub fs.FS, dst string) error {
	return fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == ".template" {
				return fs.SkipDir
			}
			return nil
		}
		if junkNames[d.Name()] {
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s already exists; rerun with --force to overwrite", target)
		}
		return nil
	})
}

func matchesRaw(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, rel); ok {
			return true
		}
		if ok, _ := path.Match(pattern, path.Base(rel)); ok {
			return true
		}
	}
	return false
}

func isBinary(data []byte) bool {
	const sniffLen = 8000
	if len(data) > sniffLen {
		data = data[:sniffLen]
	}
	return bytes.IndexByte(data, 0) >= 0
}

// outputMode picks permissions for a generated file. Embedded templates lose
// their mode bits (embed.FS reports 0444), so *.sh files are made executable
// by convention.
func outputMode(srcPath string, d fs.DirEntry) fs.FileMode {
	if strings.HasSuffix(srcPath, ".sh") {
		return 0o755
	}
	if info, err := d.Info(); err == nil && info.Mode().Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

// runHook executes .template/<scriptName> with workDir as the working
// directory. The script is staged to a temp file because the template may live
// in an embedded or remote fs.FS with no on-disk path.
func runHook(sub fs.FS, scriptName, workDir string) error {
	content, err := fs.ReadFile(sub, path.Join(".template", scriptName))
	if err != nil {
		return nil // no such hook
	}

	tmp, err := os.CreateTemp("", "gallium-hook-*.sh")
	if err != nil {
		return fmt.Errorf("failed to stage hook %s: %w", scriptName, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to stage hook %s: %w", scriptName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to stage hook %s: %w", scriptName, err)
	}

	cmd := exec.Command("sh", tmp.Name())
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("Running:", scriptName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s failed: %w", scriptName, err)
	}
	return nil
}
