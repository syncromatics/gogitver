# gogitver

gogitver is a tool to determine the semantic version of a project based on keywords used in the commit history. This project draws a lot of inspiration from [GitVersion](https://github.com/GitTools/GitVersion) but with the benefit of go's single binary executable. With the work done by go-git the binary produced can run on Linux, Windows, and Mac.

## Getting Started

### Installing

To install download the latest release from the [releases](https://github.com/annymsMthd/gogitver/releases) page for your machine architecture and place the binary in your path. You can then run the executable while in the path of your project and it should output the current version. You can then use this version to tag container images, helm charts, etc.

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

You can also override the name and location of this file by providing the settings flag ```gotgitver --settings=./anotherfile.yaml```

## Development

### Requirements

This project requires at least [Go](https://golang.org/dl/) 1.11 because it makes use of go modules for dependencies. 

### Building

To build the project simply run ```make build``` which will generate the binaries and put them in the artifacts folder.

## Built With

* [go-git](https://github.com/src-d/go-git) - The git interface

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details

## Acknowledgments

* [go-git](https://github.com/src-d/go-git) for allowing interactions with git to be easy and without dependencies
* [GitVersion](https://github.com/GitTools/GitVersion) for the inspiration
* [Visual Studio Code](https://code.visualstudio.com/) for just being an all around great editor
