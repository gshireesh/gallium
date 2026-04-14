package cmd

import "testing"

func TestReleaseAssetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", want: "gallium_darwin_arm64"},
		{name: "linux amd64", goos: "linux", goarch: "amd64", want: "gallium_linux_amd64"},
		{name: "unsupported os", goos: "windows", goarch: "amd64", wantErr: true},
		{name: "unsupported arch", goos: "darwin", goarch: "386", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := releaseAssetName(tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("releaseAssetName(%q, %q) error = nil, want error", tc.goos, tc.goarch)
				}
				return
			}

			if err != nil {
				t.Fatalf("releaseAssetName(%q, %q) error = %v", tc.goos, tc.goarch, err)
			}

			if got != tc.want {
				t.Fatalf("releaseAssetName(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

func TestLatestReleaseAssetURL(t *testing.T) {
	t.Parallel()

	got, err := latestReleaseAssetURL("darwin", "arm64")
	if err != nil {
		t.Fatalf("latestReleaseAssetURL returned error: %v", err)
	}

	want := "https://github.com/gshireesh/gallium/releases/latest/download/gallium_darwin_arm64"
	if got != want {
		t.Fatalf("latestReleaseAssetURL = %q, want %q", got, want)
	}
}
