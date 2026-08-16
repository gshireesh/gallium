package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"shireesh.com/gallium/internal/generator"
	"shireesh.com/gallium/internal/source"
)

var (
	embeddedTemplates fs.FS
	templateFlag      string
	projectNameFlag   string
	forceFlag         bool
	noHooksFlag       bool
	updateFlag        bool
)

func init() {
	rootCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Template reference (name, source/name, git URL, or local path)")
	rootCmd.Flags().StringVarP(&projectNameFlag, "name", "n", "", "Project name (directory to generate in)")
	rootCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing files in the destination")
	rootCmd.Flags().BoolVar(&noHooksFlag, "no-hooks", false, "Skip pre.sh/post.sh template hooks")
	rootCmd.Flags().BoolVarP(&updateFlag, "update", "u", false, "Update gallium to the latest release (same as gallium update)")
	rootCmd.AddCommand(listCmd)
}

var rootCmd = &cobra.Command{
	Use:           "gallium",
	Short:         "Scaffold new projects from templates with hooks",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateFlag {
			return updateBinary(cmd.OutOrStdout())
		}
		return runGenerator()
	},
}

func Execute(templates fs.FS) {
	embeddedTemplates = templates
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates from all sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := source.LoadConfig()
		if err != nil {
			return err
		}
		entries, warnings := source.List(embeddedTemplates, cfg)
		for _, w := range warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TEMPLATE\tSOURCE\tDESCRIPTION")
		for _, e := range entries {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, e.Source, e.Description)
		}
		return tw.Flush()
	},
}

func inputPrompt(label string) (string, error) {
	prompt := promptui.Prompt{Label: label}
	return prompt.Run()
}

func pickTemplate(entries []source.Entry) (string, error) {
	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = e.Name
		if e.Description != "" {
			labels[i] += " — " + e.Description
		}
	}
	prompt := promptui.Select{Label: "Select a template", Items: labels, Size: 12}
	i, _, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return entries[i].Name, nil
}

func runGenerator() error {
	cfg, err := source.LoadConfig()
	if err != nil {
		return err
	}

	ref := templateFlag
	if ref == "" {
		entries, warnings := source.List(embeddedTemplates, cfg)
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
		if len(entries) == 0 {
			return fmt.Errorf("no templates found")
		}
		ref, err = pickTemplate(entries)
		if err != nil {
			return err
		}
	}

	resolved, err := source.Resolve(embeddedTemplates, cfg, ref)
	if err != nil {
		return err
	}

	projectPath := projectNameFlag
	if projectPath == "" {
		projectPath, err = inputPrompt("Enter project name")
		if err != nil {
			return err
		}
	}
	projectName := deriveProjectName(projectPath)

	opts := generator.Options{
		Vars: map[string]string{
			"ProjectName": projectName,
			"projectName": projectName,
		},
		Force:   forceFlag,
		NoHooks: noHooksFlag,
	}
	if err := generator.Generate(resolved.FS, resolved.Template, projectPath, opts); err != nil {
		return err
	}

	if err := writeStamp(projectPath, resolved); err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to write .gallium.yaml:", err)
	}
	return nil
}

// deriveProjectName maps the destination path to the ProjectName template
// variable: the base name of the target directory.
func deriveProjectName(projectPath string) string {
	if projectPath == "" || projectPath == "." || projectPath == "./" {
		if cwd, err := os.Getwd(); err == nil {
			return filepath.Base(cwd)
		}
		return "project"
	}
	return filepath.Base(strings.TrimSuffix(filepath.Clean(projectPath), "/"))
}

// writeStamp records which template (and version) generated the project, so a
// future `gallium upgrade` can re-render against a newer tag.
func writeStamp(dst string, resolved *source.Resolved) error {
	stamp := struct {
		Template string `yaml:"template"`
		Version  string `yaml:"version,omitempty"`
		Gallium  string `yaml:"gallium"`
	}{
		Template: resolved.Ref,
		Version:  resolved.Version,
		Gallium:  currentVersion(),
	}
	data, err := yaml.Marshal(stamp)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Clean(dst), ".gallium.yaml"), data, 0o644)
}
