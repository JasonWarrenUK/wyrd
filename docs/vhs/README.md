# VHS Tape Files

[VHS](https://github.com/charmbracelet/vhs) records terminal sessions as GIF animations from declarative `.tape` scripts.

## Prerequisites

Install VHS:

    go install github.com/charmbracelet/vhs@latest

## Usage

Run a tape:

    vhs docs/vhs/<name>.tape

Or regenerate all demos:

    make demo

## Convention

Each `.tape` file in this directory produces one GIF in `docs/vhs/output/`. Tape files are checked into version control; generated GIFs are not.
