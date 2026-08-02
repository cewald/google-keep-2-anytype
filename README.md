# keep2Anytype

Go CLI for converting Google Keep Takeout exports into Any-Block JSON files for
import into Anytype.io.

Export your Google Keep data from [Google Takeout](https://takeout.google.com/settings/takeout) and unzip it.

## Install or Build

You have two options:

1. Download the binary for your operating system from the project's GitHub Releases page, extract it, use it or place it on your `PATH` to use it.
2. Build it locally with Go:

```sh
go build -o keep2anytype .
```

## Releases

Push commits to `main` using Conventional Commits such as `feat: add X` or
`fix: handle empty notes`. The release workflow creates the next semantic
version tag directly, without opening a PR, and runs GoReleaser. It builds
Linux, Windows, and macOS binaries for amd64 and arm64 and publishes SHA-256
checksums.

## Usage

```sh
./keep2anytype -p "<path-to-google-keep-export>" -o "<output-folder>"
```

## CLI Options

* `-p` - Path containing Google Keep `.json` files.
* `-o` - Output directory. It is created if necessary and must differ from the input directory.
* `-a` - Include archived notes. Defaults to `false`.
* `-m` - Conversion mode: `mixed` (default) or `pages`. Mixed mode uses pages for titled notes and notes for untitled notes. Pages mode converts every note to a page.

## Import

In Anytype, select `File -> Import -> Any-Block` and choose the output folder.
Import the complete folder so generated tag relation options are imported with
the notes.

## Notes

* Imports Google Keep labels as Anytype tags
* Does not import Google Keep images
* Does not import Google Keep note colors
* Modifies the created and modified dates to match the Google Keep note
* If the Keep note does not have a title, it uses the created date as the title
* Automatically parses any hyperlinks or annotations
