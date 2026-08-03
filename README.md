# Google Keep Converter

Go CLI for converting Google Keep Takeout exports into either Memos API payloads
or Anytype Any-Block JSON files.

Export your Google Keep data from [Google Takeout](https://takeout.google.com/settings/takeout) and unzip it.

## Install or Build

You have two options:

1. Download the binary for your operating system from the project's GitHub Releases page, extract it, use it or place it on your `PATH` to use it.
2. Build it locally with Go:

```sh
go build -o keepconv .
```

## Releases

Push commits to `main` using Conventional Commits such as `feat: add X` or
`fix: handle empty notes`. The release workflow creates the next semantic
version tag directly, without opening a PR, and runs GoReleaser. It builds
Linux, Windows, and macOS binaries for amd64 and arm64 and publishes SHA-256
checksums.

## Usage

```sh
./keepconv "<path-to-google-keep-export>" "<output-folder>"
```

The default output format is `memos`. The output directory is created if it
does not exist.

## CLI Options

* First argument - Path containing Google Keep `.json` files.
* Second argument - Output directory. It is created if necessary and must differ from the input directory.
* `-a` - Include archived notes. Defaults to `false`.
* `-format` - Output format: `memos` (default) or `anytype`.
* `-host` - Memos instance URL. Enables importing into the running instance.
* `-access-token` - Memos access token. Required with `-host`.

`MEMOS_HOST` and `MEMOS_ACCESS_TOKEN` can be used instead of the flags. The
host should be the instance URL, such as `https://memos.example.com`.

## Memos

Generate local Memos request payloads:

```sh
./keepconv -format memos "<keep-export>" "<output-folder>"
```

Import directly into a running Memos instance:

```sh
./keepconv \
  -format memos \
  -host "https://memos.example.com" \
  -access-token "$MEMOS_ACCESS_TOKEN" \
  "<keep-export>" "<output-folder>"
```

The tool POSTs each memo to `/api/v1/memos` using Bearer authentication. Pinned
notes are explicitly updated after creation because some Memos versions do not
apply `pinned` during creation.

## Anytype

Generate Anytype Any-Block files:

```sh
./keepconv -format anytype "<keep-export>" "<output-folder>"
```

Import the complete output directory in Anytype using `File -> Import ->
Any-Block`. Keep labels are emitted as Anytype tag relation options and linked
to the generated pages.

## Notes

* Memos mode imports Google Keep labels as Markdown tags
* Anytype mode imports Google Keep labels as tag relations
* Memos mode explicitly pins Keep-pinned notes after creation
* Does not import Google Keep images
* Does not import Google Keep note colors
* Modifies the created and modified dates to match the Google Keep note
* Converts annotations and checklist items to Markdown
