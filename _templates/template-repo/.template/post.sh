#!/bin/sh
if [ ! -d .git ]; then
  git init -q
  git add -A
  git commit -qm "scaffold template repo with gallium" || true
fi
echo ""
echo "Template repo ready. Next steps:"
echo "  1. Rename example-template/ and edit its files"
echo "  2. Register it:  add this repo to ~/.config/gallium/config.yaml"
echo "  3. Version it:   git tag v1.0.0 (gallium resolves the highest semver tag)"
