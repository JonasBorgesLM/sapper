module github.com/JonasBorgesLM/sapper

go 1.25.0

// The `go` directive above is the minimum LANGUAGE version — the floor Sapper
// promises. The `toolchain` below is the version CI and contributors build
// with; it is a 1.25.x patch, not the newest release, on purpose: golangci-lint
// typechecks against the stdlib of the toolchain it was itself built with and
// refuses a module targeting a newer one, failing with an `export data version`
// error that looks like a stdlib bug. Keep this at or below the CI lint pin
// (LINT_GO_VERSION in .github/workflows/ci.yml). Run lint locally with
// `GOTOOLCHAIN=go1.25.14 golangci-lint run ./...` when your local Go is newer.
toolchain go1.25.14
