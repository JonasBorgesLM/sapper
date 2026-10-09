module github.com/JonasBorgesLM/sapper

go 1.25.0

// The `go` directive above is the minimum LANGUAGE version — the floor Sapper
// promises. The `toolchain` below is the version a build actually uses: with
// the default GOTOOLCHAIN=auto, an older local Go downloads it, so it is what
// keeps a `go install` of Sapper off a vulnerable standard library. It is
// 1.27.2 for GO-2026-6617 (net/http HTTP/2; fixed in 1.26.9 and 1.27.2, and
// only 1.27.2 excludes both lines' vulnerable releases); the 1.25 line it was
// on no longer receives fixes at all. golangci-lint typechecks against the
// stdlib of the toolchain it was built with, so keep this equal to the CI lint
// pin (LINT_GO_VERSION in .github/workflows/ci.yml), and run lint locally with
// `GOTOOLCHAIN=go1.27.2 golangci-lint run ./...` when your local Go differs.
toolchain go1.27.2

require gopkg.in/yaml.v3 v3.0.1
