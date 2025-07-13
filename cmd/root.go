package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manifoldco/promptui"

	"shireesh.com/gallium/internal/generator"
	"shireesh.com/gallium/internal/tui"
)

const (
	TemplatePath = "~/gallium/templates"
)

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
	base := TemplatePath
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

	projectName := inputPrompt("Enter project name")

	vars := map[string]string{"ProjectName": projectName}
	err = generator.Generate(tplName, projectName, base, vars)
	if err != nil {
		panic(err)
	}
}
