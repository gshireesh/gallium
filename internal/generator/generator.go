package generator

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

func Generate(templateName, projectName, baseTemplateDir string, vars map[string]string) error {
	src := filepath.Join(baseTemplateDir, templateName)
	dst := filepath.Join(".", projectName)

	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}

	dotTemplateSrc := filepath.Join(src, ".template")

	if err := runHook(dotTemplateSrc, "pre.sh", dst); err != nil {
		return err
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

func runHook(templatePath, scriptName, workDir string) error {
	script := filepath.Join(templatePath, ".template", scriptName)
	fmt.Printf("Running hook script %s in %s from %s\n", scriptName, workDir, templatePath)
	if _, err := os.Stat(script); err == nil {
		cmd := exec.Command("bash", script)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("Running:", scriptName)
		return cmd.Run()
	}
	return nil
}
