package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	Use:   "gallium",
	Short: "Scaffold new projects from templates with hooks",
	Run: func(cmd *cobra.Command, args []string) {
		runGenerator()
	},
}

func Execute(templatesPath string) {
	rootCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Template name")
	rootCmd.Flags().StringVarP(&projectNameFlag, "name", "n", "", "Project name (directory to generate in)")
	TemplatesPath = templatesPath
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func inputPrompt(label string) string {
	prompt := promptui.Prompt{Label: label}
	result, err := prompt.Run()
	if err != nil {
		panic(err)
	}
	return result
}

func selectPrompt(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	return result, err
}

func runGenerator() {
	base, err := expandPath(TemplatesPath)
	if err != nil {
		panic(fmt.Errorf("failed to expand template path: %w", err))
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		panic(fmt.Errorf("failed to read template directory: %w", err))
	}
	var templates []string
	for _, e := range entries {
		if e.IsDir() {
			templates = append(templates, e.Name())
		}
	}

	tplName := templateFlag
	if tplName == "" {
		tplName, err = selectPrompt("Select a template", templates)
		if err != nil {
			panic(err)
		}
	}

	projectPath := projectNameFlag
	if projectPath == "" {
		projectPath = inputPrompt("Enter project name")
	}
	projectName := filepath.Clean(projectPath)

	if projectPath == "./" || projectPath == "." || projectPath == "" {
		// Use current directory name
		currentDirPath, err := os.Getwd()
		if err != nil {
			panic(fmt.Errorf("failed to get current working directory: %w", err))
		}
		projectName = filepath.Base(currentDirPath)
	} else {
		// pick the last part of the path as project name
		if filepath.IsAbs(projectPath) {
			projectName = filepath.Base(projectPath)
		} else {
			projectName = filepath.Clean(projectPath)
			if strings.HasSuffix(projectName, "/") {
				projectName = strings.TrimSuffix(projectName, "/")
			}
			projectName = filepath.Base(projectName)
		}
	}

	vars := map[string]string{"ProjectName": projectName}
	err = generator.Generate(tplName, projectPath, base, vars)
	if err != nil {
		panic(err)
	}
}
