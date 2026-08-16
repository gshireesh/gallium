# {{ .ProjectName }}

A [gallium](https://github.com/gshireesh/gallium) template repo. Each top-level
directory is a template; `example-template/` shows the layout.

## Template layout

Files are rendered with Go template syntax and receive the `ProjectName` /
`projectName` variables plus any defaults from metadata. An optional
`.template/` directory inside each template (never copied to output) holds:

- `metadata.yaml` — `description`, default `data` variables, and `raw` glob
  patterns for files that must be copied verbatim
- `pre.sh` / `post.sh` — hooks run in the destination directory

## Using this repo

Register it in `~/.config/gallium/config.yaml`:

```yaml
sources:
  - name: private
    path: /path/to/this/checkout      # local, always fresh
    # repo: git@github.com:you/{{ .ProjectName }}.git   # or via git once pushed
```

Then:

```bash
gallium list
gallium -t private/example-template -n my-app
gallium -t private/example-template@v1 -n my-app   # pin a tag (repo sources only)
```

## Versioning

Tag releases with semver (`git tag v1.0.0 && git push --tags`). For sources
registered with `repo:`, gallium resolves the highest matching semver tag;
`path:` sources always use the working tree.
