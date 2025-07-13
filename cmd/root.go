package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/manifoldco/promptui"

	"shireesh.com/gallium/internal/generator"
	"shireesh.com/gallium/internal/tui"
)

const (
	TemplatePath = "~/gallium/templates"
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

func Execute() {
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

func runGenerator() {
	base, err := expandPath(TemplatePath)
	if err != nil {
		panic(fmt.Errorf("failed to expand template path: %w", err))
	}
	entries, _ := os.ReadDir(base)
	var templates []string
	for _, e := range entries {
		if e.IsDir() {
			templates = append(templates, e.Name())
		}
	}
	tplName, err := tui.SelectTemplate(templates)
	if err != nil {
		panic(err)
	}

	projectPath := inputPrompt("Enter project name")
	projectName := filepath.Clean(projectPath)

	if projectPath == "./" || projectPath == "." || projectPath == "" {
		// set projectName to current directory name
		currentDirPath, err := os.Getwd()
		if err != nil {
			panic(fmt.Errorf("failed to get current working directory: %w", err))
		}
		currentDir := filepath.Base(currentDirPath)
		projectName = currentDir
	}

	vars := map[string]string{"ProjectName": projectName}
	err = generator.Generate(tplName, projectPath, base, vars)
	if err != nil {
		panic(err)
	}
}
