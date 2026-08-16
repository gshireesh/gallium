# Gallium

Gallium is a personal scaffolding CLI. Templates can come from three kinds of
sources: the set embedded in this repo, local directories, and git
repositories (public or private).

## Install

With curl:

```bash
curl -fsSL https://raw.githubusercontent.com/gshireesh/gallium/master/install.sh | sh
```

With Go:

```bash
go install shireesh.com/gallium@latest
```

From a local checkout:

```bash
make install
```

## Update

```bash
gallium update   # or: gallium -u
```

`gallium update` downloads the latest release binary from GitHub Releases and replaces the current executable.

## Usage

```bash
gallium                                    # interactive picker across all sources
gallium list                               # list templates from all sources
gallium -t python-dev -n my-app            # embedded template
gallium -t private/wordpress -n my-app     # named source from config
gallium -t private/wordpress@v1 -n my-app  # pin a tag (git sources)
gallium -t github.com/x/tpl -n my-app      # ad-hoc git repo (highest semver tag)
gallium -t github.com/x/tpls//py -n my-app # subdirectory of a git repo
gallium -t ./some-template -n my-app       # local directory
gallium -t python-dev -n my-app --force    # overwrite existing files
gallium -t python-dev -n my-app --no-hooks # skip pre.sh/post.sh
gallium version
```

Generation refuses to overwrite existing files unless `--force` is given; the
conflict check runs before any hook, so a refused run leaves the destination
untouched. Every generated project gets a `.gallium.yaml` recording the
template reference and resolved version.

## Sources

Named sources live in `~/.config/gallium/config.yaml`:

```yaml
sources:
  - name: private
    repo: git@github.com:gshireesh/private-templates.git  # your git auth applies
  - name: work
    path: ~/work/templates
```

Cloning uses the git CLI, so private repos work with your existing SSH/HTTPS
credentials — gallium holds no tokens. A repo whose root contains `.template/`
is a single template (`-t private`); otherwise each top-level directory is a
template (`-t private/<name>`).

**Versioning:** tag template repos with semver (`git tag v1.2.0`). Gallium
resolves `@v1` to the highest matching tag, no `@` to the highest tag overall,
and falls back to the default branch for untagged repos. Pinned tags are cached
in `~/Library/Caches/gallium` (override with `GALLIUM_CACHE_DIR`); branches are
re-fetched each run.

To start a new template repo: `gallium -t template-repo -n my-templates`.

## Authoring Templates

Files are rendered with Go `text/template` and receive `ProjectName`/`projectName`
plus any defaults from metadata. Binary files are copied verbatim, and `*.sh`
files are made executable in the output.

An optional `.template/` directory at the template root (never copied into the
output) can contain:

- `pre.sh` / `post.sh` — hooks run with the destination directory as the working directory, before and after files are generated.
- `metadata.yaml`:

  ```yaml
  description: Shown by gallium list
  data:            # default template variables (CLI vars win)
    projectAuthor: Shireesh Kumar G
  raw:             # glob patterns copied verbatim, for files with literal {{ }}
    - "*.yml.j2"
  ```

Embedded templates live in `_templates/` (underscore-prefixed so Go tooling
ignores template source code). Gotcha for Go templates: a file named `go.mod`
prevents `go:embed` from including the template in the binary. Name it
`go.mod.tpl` and rename it in `post.sh` (see `_templates/go-basic`).

## Release Flow

Pushing to `master` with `release:` in the commit message creates a new tag and GitHub Release.
The release workflow builds platform binaries and uploads them as release assets for `install.sh` and `gallium update`.
