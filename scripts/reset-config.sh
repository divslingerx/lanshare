#!/bin/bash
# Deletes the filehub config and manifests so the app starts fresh.
# Run this when the app is not running.

dir="${XDG_CONFIG_HOME:-$HOME/.config}/filehub"

if [[ "$OSTYPE" == "darwin"* ]]; then
  dir="$HOME/Library/Application Support/filehub"
fi

if [ ! -d "$dir" ]; then
  echo "Nothing to delete — $dir does not exist."
  exit 0
fi

echo "Deleting: $dir"
rm -rf "$dir"
echo "Done. filehub will start fresh on next launch."
