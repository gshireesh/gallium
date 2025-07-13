package generator

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// GetVarsFromMetadata reads a metadata YAML file and returns a map of variables under the "data" key.
func GetVarsFromMetadata(metadataPath string) (map[string]string, error) {
	file, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(file, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	return meta.Data, nil
}

func Generate(templateName, projectName, baseTemplateDir string, vars map[string]string) error {
	src := filepath.Join(baseTemplateDir, templateName)
	dst := filepath.Join(".", projectName)

	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}

	if err := runHook(src, "pre.sh", dst); err != nil {
		return err
	}

	metadataPath := filepath.Join(src, ".template", "metadata.yaml")
	if _, err := os.Stat(metadataPath); err == nil {
		metadataVars, err := GetVarsFromMetadata(metadataPath)
		if err != nil {
			return fmt.Errorf("failed to get variables from metadata: %w", err)
		}
		for k, v := range metadataVars {
			if _, exists := vars[k]; !exists {
				vars[k] = v
			}
		}
	}

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if strings.Contains(path, ".template") || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tpl, err := template.New(rel).Parse(string(data))
		if err != nil {
			return err
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		defer f.Close()
		return tpl.Execute(f, vars)
	})
	if err != nil {
		return err
	}

	return runHook(src, "post.sh", dst)
}

func makeScriptExecutable(scriptPath string) error {
	if err := os.Chmod(scriptPath, 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}
	return nil
}

func runHook(templatePath, scriptName, workDir string) error {
	script := filepath.Join(templatePath, ".template", scriptName)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	script = filepath.Join(cwd, script)
	if _, err := os.Stat(script); err == nil {
		if err := makeScriptExecutable(script); err != nil {
			return fmt.Errorf("failed to make script executable: %w", err)
		}
		cmd := exec.Command("bash", "-c", script)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("Running:", scriptName)
		return cmd.Run()
	}
	return nil
}
