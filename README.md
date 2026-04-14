# Gallium

Gallium is a personal scaffolding CLI for bootstrapping the templates in this repo.

## Install

With curl:

```bash
curl -fsSL https://raw.githubusercontent.com/gshireesh/gallium/master/install.sh | sh
```

With Go:

```bash
go install shireesh.com/gallium@latest
```

## Update

```bash
gallium update
```

`gallium update` downloads the latest release binary from GitHub Releases and replaces the current executable.

## Usage

```bash
gallium
gallium -t python-dev -n my-app
gallium version
```

## Release Flow

Pushing to `master` with `release:` in the commit message creates a new tag and GitHub Release.
The release workflow builds platform binaries and uploads them as release assets for `install.sh` and `gallium update`.