# `tsukuru`: build more than just a binary

Tsukuru is a cli tool that can build and run apk and appbundle for deploying Go apps on android platforms, and wasm for web targets.

```
~ go install github.com/rajveermalviya/tsukuru@latest

~ tsukuru -help

Usage of tsukuru:

        tsukuru build {apk, appbundle, wasm} [-options] <path to main package>

        tsukuru run {apk, wasm} [-options] <path to main package>

        tsukuru checkin deps [-options] <path to main package>

Run 'tsukuru [command] [subcommand] -help' for details
```

# android backends
`tsukuru` currently has two backends for android build system.

- `gradle` (recommended)

- `custom` (experimental) : custom backend can build apks without running gradle, though it is limited in many cases (doesn't support building appbundle, doesn't support building apps with android dependencies)

# `tsukurufile` (experimental)

`tsukurufile` can be used to specify android dependencies for a go package

```go
tsukuru v1alpha

android (
    "androidx.games:games-activity:1.2.1"
)
```

`tsukuru` walks through each imported package's directory and tries to find a `tsukurufile`, then it deduplicates any duplicate dependencies between different `tsukurufile`'s (currently doesn't do any version management), then it adds the unique list of dependencies to your `./android/app/build.gradle` file.
