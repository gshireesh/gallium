package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	releaseOwner = "gshireesh"
	releaseRepo  = "gallium"
)

var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the installed gallium version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), currentVersion())
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and install the latest gallium release",
	RunE: func(cmd *cobra.Command, args []string) error {
		return updateBinary(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
}

func currentVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		return buildInfo.Main.Version
	}

	return "dev"
}

func releaseAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "darwin", "linux":
	default:
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}

	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	return fmt.Sprintf("gallium_%s_%s", goos, goarch), nil
}

func latestReleaseAssetURL(goos, goarch string) (string, error) {
	assetName, err := releaseAssetName(goos, goarch)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s", releaseOwner, releaseRepo, assetName), nil
}

func executablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err == nil {
		return resolvedPath, nil
	}

	return executablePath, nil
}

func updateBinary(out io.Writer) error {
	currentExecutable, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}

	assetURL, err := latestReleaseAssetURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to prepare update download: %w", err)
	}
	req.Header.Set("User-Agent", "gallium/"+currentVersion())

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download latest release: %s", resp.Status)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(currentExecutable), ".gallium-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", filepath.Dir(currentExecutable), err)
	}

	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write downloaded release: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to finalize downloaded release: %w", err)
	}

	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to mark downloaded release executable: %w", err)
	}

	if err := os.Rename(tempPath, currentExecutable); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			return fmt.Errorf("failed to replace %s: %w; rerun with permissions for that directory", currentExecutable, err)
		}
		return fmt.Errorf("failed to replace %s: %w", currentExecutable, err)
	}

	fmt.Fprintf(out, "Updated gallium at %s\n", currentExecutable)
	return nil
}
