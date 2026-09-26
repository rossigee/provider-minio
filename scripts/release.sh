#!/bin/bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

if [[ $# -ne 1 ]]; then
    printf 'Usage: %s vMAJOR.MINOR.PATCH\n' "$0" >&2
    exit 1
fi

version="$1"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'Version must match vMAJOR.MINOR.PATCH.\n' >&2
    exit 1
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd -- "$script_dir/.."

git fetch origin master

worktree_status="$(git status --porcelain)"
if [[ -n "$worktree_status" ]]; then
    printf 'Working tree must be clean.\n' >&2
    exit 1
fi

head_commit="$(git rev-parse --verify 'HEAD^{commit}')"
origin_master_commit="$(git rev-parse --verify 'refs/remotes/origin/master^{commit}')"
if [[ "$head_commit" != "$origin_master_commit" ]]; then
    printf 'HEAD must match origin/master.\n' >&2
    exit 1
fi

if [[ -f VERSION ]]; then
    version_file="$(<VERSION)"
    if [[ "$version_file" != "$version" ]]; then
        printf 'VERSION must match %s.\n' "$version" >&2
        exit 1
    fi
fi

if git show-ref --verify --quiet "refs/tags/$version"; then
    printf 'Tag already exists locally: %s\n' "$version" >&2
    exit 1
fi

remote_tags="$(git ls-remote --tags origin "refs/tags/$version")"
if [[ -n "$remote_tags" ]]; then
    printf 'Tag already exists on origin: %s\n' "$version" >&2
    exit 1
fi

git tag -a "$version" -m "Release $version" HEAD
git push origin "refs/tags/$version:refs/tags/$version"
