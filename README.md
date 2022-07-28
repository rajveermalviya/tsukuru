# `tsukuru`: build more than just a binary

Tsukuru is a library that can build apk and appbundle for deploying Go apps on android platforms.

```
~ go install github.com/rajveermalviya/tsukuru@latest

~ tsukuru -help

Usage of tsukuru:

        tsukuru build {apk, appbundle} [-options] <path to main package>

        tsukuru run apk [-options] <path to main package>

Run 'tsukuru [command] [subcommand] -help' for details
```

### But how does it differ from `gomobile`?

`tsukuru` doesn't generate android source code by itself, it instead requires an `android` directory that contains all the java/kotlin code. This make it possible to add android dependencies and supporting `androidx` libraries. So `tsukuru` is much more flexible than `gomobile`.

# backends
`tsukuru` currently has two backends for android build system.

- `gradle` (recommended)

- `custom` (experimental) : custom backend can build apks without running gradle, though it is limited in many cases (doesn't support building appbundle, doesn't support building apps with android dependencies)
