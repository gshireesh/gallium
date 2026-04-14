package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"shireesh.com/gallium/internal/generator"
)

var (
	TemplatesPath   string
	templateFlag    string
	projectNameFlag string
)

func init() {
	rootCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Template name")
	rootCmd.Flags().StringVarP(&projectNameFlag, "name", "n", "", "Project name (directory to generate in)")
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}

var rootCmd = &cobra.Command{
	Use:          "gallium",
	Short:        "Scaffold new projects from templates with hooks",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerator()
	},
}

func Execute(templatesPath string) {
	TemplatesPath = templatesPath
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func inputPrompt(label string) (string, error) {
	prompt := promptui.Prompt{Label: label}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}

func selectPrompt(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	return result, err
}

func runGenerator() error {
	base, err := expandPath(TemplatesPath)
	if err != nil {
		return fmt.Errorf("failed to expand template path: %w", err)
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("failed to read template directory: %w", err)
	}

	var templates []string
	for _, e := range entries {
		if e.IsDir() {
			templates = append(templates, e.Name())
		}
	}
	sort.Strings(templates)
	if len(templates) == 0 {
		return fmt.Errorf("no templates found in %s", base)
	}

	tplName := templateFlag
	if tplName == "" {
		tplName, err = selectPrompt("Select a template", templates)
		if err != nil {
			return err
		}
	} else if !containsTemplate(templates, tplName) {
		return fmt.Errorf("template %q not found", tplName)
	}

	projectPath := projectNameFlag
	if projectPath == "" {
		projectPath, err = inputPrompt("Enter project name")
		if err != nil {
			return err
		}
	}
	projectName := filepath.Clean(projectPath)

	if projectPath == "./" || projectPath == "." || projectPath == "" {
		// Use current directory name
		currentDirPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		projectName = filepath.Base(currentDirPath)
	} else {
		// pick the last part of the path as project name
		if filepath.IsAbs(projectPath) {
			projectName = filepath.Base(projectPath)
		} else {
			projectName = filepath.Clean(projectPath)
			projectName = strings.TrimSuffix(projectName, "/")
			projectName = filepath.Base(projectName)
		}
	}

	vars := map[string]string{
		"ProjectName": projectName,
		"projectName": projectName,
	}
	if err := generator.Generate(tplName, projectPath, base, vars); err != nil {
		return err
	}

	return nil
}

func containsTemplate(templates []string, wanted string) bool {
	for _, templateName := range templates {
		if templateName == wanted {
			return true
		}
	}
	return false
}
