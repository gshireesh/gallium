package source

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// parseGitRef splits a git template reference into its repository URL, an
// optional subdirectory (after "//", ignoring a scheme's "://") and an
// optional version (after a trailing "@" that is not part of an ssh user).
//
//	github.com/x/tpl@v1          -> url=github.com/x/tpl  version=v1
//	github.com/x/tpls//py@v1.2.0 -> url=github.com/x/tpls subdir=py version=v1.2.0
//	git@github.com:x/tpl.git     -> url=git@github.com:x/tpl.git
func parseGitRef(ref string) (url, subdir, version string) {
	url = ref
	if i := strings.LastIndex(url, "@"); i > 0 && !strings.ContainsAny(url[i+1:], "/:") {
		version = url[i+1:]
		url = url[:i]
	}

	rest := url
	offset := 0
	if i := strings.Index(rest, "://"); i >= 0 {
		offset = i + 3
		rest = rest[offset:]
	}
	if i := strings.Index(rest, "//"); i >= 0 {
		subdir = strings.Trim(rest[i+2:], "/")
		url = url[:offset+i]
	}
	return url, subdir, version
}

// looksLikeGitURL reports whether ref should be treated as a git repository
// reference: an explicit scheme/ssh form, or a host-like first path segment
// (contains a dot, e.g. github.com/...).
func looksLikeGitURL(ref string) bool {
	if strings.HasPrefix(ref, "git@") || strings.Contains(ref, "://") {
		return true
	}
	first, _, found := strings.Cut(ref, "/")
	return found && strings.Contains(first, ".")
}

// cloneURL normalizes a bare host path like github.com/x/y to an https URL.
func cloneURL(url string) string {
	if strings.HasPrefix(url, "git@") || strings.Contains(url, "://") {
		return url
	}
	if strings.Contains(strings.SplitN(url, "/", 2)[0], ".") {
		return "https://" + url
	}
	return url // local path used as a repo (tests, file-based remotes)
}

var semverTagRe = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$`)

type semver struct {
	tag     string
	parts   [3]int
	defined int // how many components the tag spells out
}

func parseSemverTag(tag string) (semver, bool) {
	m := semverTagRe.FindStringSubmatch(tag)
	if m == nil {
		return semver{}, false
	}
	v := semver{tag: tag}
	for i, s := range m[1:] {
		if s == "" {
			break
		}
		v.parts[i], _ = strconv.Atoi(s)
		v.defined = i + 1
	}
	return v, true
}

func (v semver) less(o semver) bool {
	for i := range 3 {
		if v.parts[i] != o.parts[i] {
			return v.parts[i] < o.parts[i]
		}
	}
	return false
}

// matchesPrefix reports whether v satisfies a partial request like v1 or v1.2.
func (v semver) matchesPrefix(req semver) bool {
	for i := 0; i < req.defined; i++ {
		if v.parts[i] != req.parts[i] {
			return false
		}
	}
	return true
}

// listRemoteTags returns the tag names advertised by the repository.
func listRemoteTags(url string) ([]string, error) {
	out, err := exec.Command("git", "ls-remote", "--tags", cloneURL(url)).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-remote %s: %s", url, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git ls-remote %s: %w", url, err)
	}

	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		_, ref, found := strings.Cut(line, "refs/tags/")
		if !found {
			continue
		}
		tags = append(tags, strings.TrimSuffix(ref, "^{}"))
	}
	return tags, nil
}

// pickVersion resolves a requested version against a tag list. An empty
// request picks the highest semver tag (or "" meaning default branch when the
// repo has no semver tags). A partial request like v1 or v1.2 picks the
// highest matching tag. An exact tag or anything non-semver (a branch name)
// is returned as-is.
func pickVersion(tags []string, requested string) (string, error) {
	for _, t := range tags {
		if t == requested && requested != "" {
			return requested, nil
		}
	}

	req, reqIsSemver := parseSemverTag(requested)
	if requested != "" && !reqIsSemver {
		return requested, nil // branch or other ref; let git resolve it
	}

	var best *semver
	for _, t := range tags {
		v, ok := parseSemverTag(t)
		if !ok {
			continue
		}
		if reqIsSemver && !v.matchesPrefix(req) {
			continue
		}
		if best == nil || best.less(v) {
			b := v
			best = &b
		}
	}
	if best != nil {
		return best.tag, nil
	}
	if requested != "" {
		return "", fmt.Errorf("no tag matches %q", requested)
	}
	return "", nil // no semver tags: use the default branch
}

func cacheRoot() (string, error) {
	if dir := os.Getenv("GALLIUM_CACHE_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gallium"), nil
}

var unsafePathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// materialize ensures a shallow clone of url at version exists locally and
// returns its directory. Semver-tag checkouts are immutable and reused from
// the cache; branches and the default branch are re-cloned each run so they
// stay fresh.
func materialize(url, version string) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	key := unsafePathChars.ReplaceAllString(url, "-") + "@" + version
	dir := filepath.Join(root, "repos", key)

	_, pinned := parseSemverTag(version)
	if pinned {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}

	args := []string{"clone", "--quiet", "--depth", "1"}
	if version != "" {
		args = append(args, "--branch", version)
	}
	args = append(args, cloneURL(url), dir)
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git clone %s failed: %w", url, err)
	}
	os.RemoveAll(filepath.Join(dir, ".git"))
	return dir, nil
}

// fetchGit resolves the version and materializes the repository, returning
// the local directory and the resolved version ("" when the default branch
// was used).
func fetchGit(url, requestedVersion string) (dir, version string, err error) {
	tags, err := listRemoteTags(url)
	if err != nil {
		return "", "", err
	}
	version, err = pickVersion(tags, requestedVersion)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", url, err)
	}
	dir, err = materialize(url, version)
	if err != nil {
		return "", "", err
	}
	return dir, version, nil
}
