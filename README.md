# `tsukuru`: build more than just a binary

Tsukuru is a library that can build apk and appbundle for deploying Go apps on android platforms.

Go's default build system (aka `go build`) is great but some deployment targets require more than just a binary, `tsukuru` provides just that, currently `tsukuru` can build apk and appbundle for android platform.

### But how does it differ from `gomobile`?

`tsukuru` doesn't generate android source code by itself, it instead requires an `android` directory that contains all the java/kotlin code. This make it possible to add android dependencies and supporting `androidx` libraries.

# backends
`tsukuru` currently has two backends for android build system.

- `gradle` (recommended)

- `custom` (experimental) : custom backend can build apks without running gradle, though it is limited in many cases. It doesn't support building appbundle and doesn't support building apps with android dependencies.
