# keep2Anytype

Go CLI for converting Google Keep Takeout exports into Any-Block JSON files for
import into Anytype.io.

Export your Google Keep data from [Google Takeout](https://takeout.google.com/settings/takeout) and unzip it.

## Build

```sh
go build -o keep2anytype .
```

The repository also includes a prebuilt Linux `keep2anytype` binary.

## Releases

Push commits to `main` using Conventional Commits such as `feat: add X` or
`fix: handle empty notes`. Release Please will open or update a release PR,
calculate the next semantic version, and create the version tag when the PR is
merged. That tag triggers GoReleaser, which builds Linux, Windows, and macOS
binaries for amd64 and arm64 and publishes SHA-256 checksums.

GitHub must allow workflows to create pull requests. Enable **Settings ->
Actions -> General -> Workflow permissions -> Allow GitHub Actions to create
and approve pull requests**.

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
