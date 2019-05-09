# Make labels for GitHub repo [![GitHub release](https://img.shields.io/github/release/powerman/gh-make-labels.svg)](https://github.com/powerman/gh-make-labels/releases/latest) [![CI Build Status](https://circleci.com/gh/powerman/gh-make-labels.svg?style=svg)](https://circleci.com/gh/powerman/gh-make-labels) [![Go Report Card](https://goreportcard.com/badge/github.com/powerman/gh-make-labels)](https://goreportcard.com/report/github.com/powerman/gh-make-labels)

This command-line tool will update given repo's labels to match predefined
labels list.

Check [this repo labels](https://github.com/powerman/gh-make-labels/labels)
as example which labels you'll have after running this tool using example
[config](./gh-labels.yml).


## Installation

Download binary for your OS manually from
[releases](https://github.com/powerman/gh-make-labels/releases) or run
this to install the latest version:

```sh
curl -sfL $(curl -s https://api.github.com/repos/powerman/gh-make-labels/releases/latest | grep -i /gh-make-labels-$(uname -s)-$(uname -m)\" | cut -d\" -f4) | sudo install /dev/stdin /usr/local/bin/gh-make-labels
```

### Dependencies

You should have installed and configured
[hub](https://github.com/github/hub) tool (gh-make-labels will use hub's
token to access GitHub API).


## Usage

Create file `gh-labels.yml` with list of labels to create and their colors
(take a look at provided [example config](./gh-labels.yml)) and run this
tool on any repo where you've admin access.

```
$ gh-make-labels -h
Usage: gh-make-labels [flags] owner/repo
  -cleanup
        delete unknown labels
  -config path
        path to config file (default "gh-labels.yml")
  -log.level level
        log level (debug|info|warn|err) (default "info")

$ curl -sfL https://raw.githubusercontent.com/powerman/gh-make-labels/master/gh-labels.yml >gh-labels.yml
$ gh-make-labels -cleanup owner/repo
inf `update` label=bug color=DC143C old_color=d73a4a
inf `remove` label=duplicate
inf `update` label=enhancement color=008000 old_color=a2eeef
inf `remove` label=good first issue
inf `remove` label=help wanted
inf `remove` label=invalid
inf `update` label=question color=0000CD old_color=d876e3
inf `remove` label=wontfix
inf `create` label=BLOCKED color=5218FA
inf `create` label=TBD color=FFFFFF
inf `create` label=URGENT color=FFC0CB
inf `create` label=blocker color=5218FA
inf `create` label=chore color=FFD700
inf `create` label=doc color=FFD700
inf `create` label=feature color=008000
inf `create` label=optimization color=008000
inf `create` label=refactoring color=FFD700
inf `create` label=test color=FFD700
inf `create` label=vulnerability color=DC143C
inf `create` label=∈API color=90EE90
inf `create` label=∈UserStory color=90EE90
inf `create` label=∈architecture color=F0E68C
inf `create` label=∈framework color=F0E68C
inf `create` label=∈security color=F0E68C
inf `create` label=⌘dev color=8A2BE2
inf `create` label=⌘production color=8A2BE2
inf `create` label=⌘staging color=8A2BE2
inf `create` label=◷16h color=E6E6FA
inf `create` label=◷1h color=E6E6FA
inf `create` label=◷3h color=E6E6FA
inf `create` label=◷8h color=E6E6FA
inf `create` label=➊ color=E6E6FA
inf `create` label=➋ color=E6E6FA
inf `create` label=➌ color=E6E6FA
inf `create` label=➎ color=E6E6FA
inf `create` label=➑ color=E6E6FA
$
```
