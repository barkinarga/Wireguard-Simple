#!/usr/bin/env bash
# Fetch one older stable release, pin both assets to that tag, verify checksum.
set -euo pipefail
mkdir -p .tools/previous
gh api --paginate repos/barkinarga/Wireguard-Simple/releases \
  --jq '.[] | select(.draft == false and .prerelease == false) | .tag_name' > .tools/previous/tags
tag=$(node --input-type=module -e '
  import fs from "node:fs";
  const version = JSON.parse(fs.readFileSync("package.json", "utf8")).version.split("-")[0].split(".").map(Number);
  const compare = (a, b) => a[0]-b[0] || a[1]-b[1] || a[2]-b[2];
  const tags = fs.readFileSync(".tools/previous/tags", "utf8").trim().split(/\s+/)
    .filter(t => /^v[0-9]+\.[0-9]+\.[0-9]+$/.test(t))
    .map(tag => ({tag, version: tag.slice(1).split(".").map(Number)}))
    .filter(t => compare(t.version, version) < 0)
    .sort((a,b) => compare(b.version, a.version));
  if (!tags.length) throw Error("No previous stable release available for upgrade validation");
  console.log(tags[0].tag);
')
gh release download "$tag" --repo barkinarga/Wireguard-Simple \
  --pattern wgsimple-install.run --pattern SHA256SUMS --dir .tools/previous
cd .tools/previous
checksum=$(awk '$2 == "wgsimple-install.run" && length($1) == 64 && $1 !~ /[^0-9a-f]/ {hash=$1; count++} END {if(count == 1) print hash; else exit 1}' SHA256SUMS)
printf '%s  wgsimple-install.run\n' "$checksum" | sha256sum --check --status
printf '%s\n' "${tag#v}" > version
printf 'Verified previous stable release: %s\n' "$tag"
