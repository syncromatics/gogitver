# gogitver

gogitver is a tool to determine the semantic version of a project based on keywords used in the commit history. This project draws a lot of inspiration from [GitVersion](https://github.com/GitTools/GitVersion) but with the benefit of go's single binary executable. With the work done by go-git the binary produced can run on Linux, Windows, and Mac.

## Getting Started

### Installing

#### Go install

If you have Go installed, install the latest tagged release directly from GitHub:

```sh
go install github.com/syncromatics/gogitver/cmd/gogitver@latest
```

For a reproducible install, use a specific version tag:

```sh
go install github.com/syncromatics/gogitver/cmd/gogitver@v1.2.0
```

Make sure Go's binary directory is on your `PATH`. This is usually `$GOPATH/bin`, or `$HOME/go/bin` when `GOPATH` is not set.

#### Binaries

To install download the latest release from the [releases](https://github.com/syncromatics/gogitver/releases) page for your machine architecture and place the binary in your path. You can then run the executable while in the path of your project and it should output the current version. You can then use this version to tag container images, helm charts, etc.

#### Snap

The Snap package is outdated and should be avoided for new installs. New versions are not currently published to Snap.

### Usage

To get this most out of this tool you should be adding keywords to your git commits.

Example: 
```git commit -m "(+semver: breaking) this change adds a breaking change to the public api"```

When gogitver sees this commit in the git history it will bump the major version.

The default regex patterns gogitver will use are:

* Major: ```\+semver:\s?(breaking|major)```
* Minor: ```\+semver:\s?(feature|minor)```
* Patch: ```\+semver:\s?(fix|patch)```

However you can override these by providing a settings file ```.gogitver.yaml``` that looks like:

```yaml
major-version-bump-message: '(major|breaking)\(.*\)'
minor-version-bump-message: '(feat|feature|minor)\(.*\)'
patch-version-bump-message: '(patch|fix)\(.*\)'
```

You can also override the name and location of this file by providing the settings flag ```gogitver --settings=./anotherfile.yaml```

### Primary branch

gogitver uses the repository's primary branch as the stable version baseline. By default it looks for `main`, `master`, `origin/main`, then `origin/master`. For a single branch name, a local ref is preferred over its remote counterpart (a local `main` is used before `origin/main`).

If both a `main` and a `master` branch exist anywhere - local or remote, in any combination - gogitver refuses to guess the version baseline and requires an explicit primary branch. This prevents it from silently switching the baseline during a branch migration:

```sh
gogitver --primary-branch=main
```

The `--primary-branch` value also takes a local ref or `origin/<name>`, so you can point the baseline at any branch, not just `main` or `master`.

Use `--forbid-behind-primary` to error when the current branch calculates a version lower than the resolved primary branch. The older `--forbid-behind-master` flag remains available as a deprecated compatibility alias.

### CI note

gogitver currently stays on `go-git` v4 for compatibility with CI checkouts that use shared or alternate git object stores. We tested `go-git` v5 during modernization, but some agent workspaces that worked with v4 failed to load primary branch commits under v5 even though native `git` commands still worked.

## Development

### Requirements

This project requires [Go](https://golang.org/dl/) 1.26.3.

### Building

To build the project simply run ```make build``` which will generate the binaries and put them in the artifacts folder.

## Built With

* [go-git](https://github.com/go-git/go-git) - The git interface

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details

## Acknowledgments

* [go-git](https://github.com/go-git/go-git) for allowing interactions with git to be easy and without dependencies
* [GitVersion](https://github.com/GitTools/GitVersion) for the inspiration
* [Visual Studio Code](https://code.visualstudio.com/) for just being an all around great editor
