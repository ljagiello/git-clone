# git-clone

A small CLI tool that clones Git repositories into a clean, consistent directory structure:

```
~/code/<host>/<org>/<repo>
```

No more `mkdir -p` before cloning. Just pass a URL in any format and the tool creates the right directories, clones the repo, and prints the resulting path.

## Usage

```sh
git-clone <repository-url>
```

The tool accepts any common Git URL format:

```sh
# HTTPS
git-clone https://github.com/org/repo.git
git-clone https://github.com/org/repo

# SSH
git-clone git@github.com:org/repo.git

# Bare (scheme is inferred)
git-clone github.com/org/repo
```

On success, the full path to the cloned repository is printed to stdout:

```
/home/user/code/github.com/org/repo
```

This makes it easy to combine with `cd`:

```sh
cd $(git-clone github.com/org/repo)
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `GIT_CLONE_ROOT` | `~/code` | Base directory for the organized tree |

## Install

```sh
go install github.com/ljagiello/git-clone@latest
```

Or build from source:

```sh
git clone https://github.com/ljagiello/git-clone.git
cd git-clone
go build -o git-clone .
```

## How it works

1. Parses the input URL (HTTPS, SSH `user@host:path`, `ssh://`, or bare `host/path`).
2. Extracts `<host>/<org>/<repo>` from the URL, normalizing the hostname to lowercase and stripping `.git` suffixes.
3. Joins the repo path with the base directory (`$GIT_CLONE_ROOT` or `~/code`).
4. Creates parent directories and runs `git clone`.
5. On failure, cleans up partial clones that are not recoverable and removes empty parent directories.

## License

[Apache License 2.0](LICENSE)
