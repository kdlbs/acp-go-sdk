# Releasing

The Go SDK and the ACP schema artifacts use independent version numbers.

- `sdk/version` contains the SDK version used for the next Go module tag, such
  as `v0.14.0`.
- `schema/version` selects the ACP schema input, such as `1.20.0`. The generator
  downloads that input from the upstream release `schema-v1.20.0`.
- The root `version` file is a generated-schema stamp. It must match
  `schema/version`; it is not the SDK version.

An SDK release can adopt a new schema or contain SDK-only fixes. Do not assume
that the two version numbers should match.

## Prerequisites

- `make`, `curl`, and `git` in your `PATH`.
- [mise](https://mise.jdx.dev) for the toolchain. Run `mise install` once to
  provision Go, `treefmt`, and the formatters used by `make fmt` and
  `make check`.
- The repository must have an Actions secret named `ANTHROPIC_API_KEY` so the
  release-notes workflow can update GitHub Release bodies after publication.

## Update the ACP Schema Input

Only perform this step when the SDK should adopt a different published ACP
schema. Pass the schema artifact version, without the `schema-v` prefix:

```bash
make update-schema SCHEMA_VERSION=1.20.0
```

This target:

- writes `1.20.0` to `schema/version`
- downloads stable and unstable inputs from the `schema-v1.20.0` release
- regenerates the Go bindings and helper APIs
- formats the repository
- updates the root `version` generated-schema stamp

Review and test schema updates before preparing an SDK release:

```bash
make test
git diff --check
git status --short
```

`make clean && make version && make fmt` should reproduce the same generated
tree. The root `version` and `schema/version` files must remain identical.

## Prepare the SDK Release

Choose the next SDK semantic version independently from the schema input. For
this release, ACP schema `schema-v1.20.0` is published as Go SDK `v0.14.0`:

```bash
make release VERSION=0.14.0
```

The release helper:

- writes `0.14.0` to `sdk/version`
- regenerates against the schema selected by `schema/version` without changing
  that selection
- runs formatting, the Go test suite, and example builds
- verifies `sdk/version` matches the requested SDK release
- verifies the generated-schema stamp matches `schema/version`

The target does not commit, tag, or publish anything. It is designed to prepare
unstaged candidate changes in a normal worktree, so it does not invoke
`make check`: that target intentionally requires no unstaged diff and remains a
clean-tree/CI gate. Run `make check` before starting a release from a clean tree,
or after committing the reviewed release candidate.

Ensure `sdk/version` is included in the release commit. It drives the README
installation example and records the SDK tag independently from the schema.

## Review and Commit

1. Inspect `git status` and `git diff`. Schema files should change only when
   `make update-schema` was run; `make release` itself changes the SDK version
   and generated documentation as needed.
1. Run `make test` and `git diff --check` on the candidate.
1. Commit with a descriptive SDK subject such as `release: v0.14.0`.
1. From the clean committed tree, run `make check` as the final formatting and
   generated-documentation gate.
1. Push the branch and open a pull request if review is required.

## Tag and Publish

Tag the SDK release commit with the SDK version, not the schema artifact tag:

```bash
git tag v0.14.0
git push origin v0.14.0
```

Create a GitHub release for `v0.14.0` and mention that it was generated from ACP
schema `schema-v1.20.0`. The upstream schema tag is an input reference and must
not be reused as the Go module tag.

After publication, CI runs `communique` and replaces the release body with
generated notes. The workflow requires a `v*` SDK release tag and the
`ANTHROPIC_API_KEY` Actions secret.

Consumers rely on the `vX.Y.Z` SDK tag for `go get`, so ensure the tag is pushed
before announcing the release.

## Additional Notes

- If a schema update introduces breaking Go API changes, update examples and
  documentation in the same release commit.
- Release-note automation updates GitHub Release bodies only; it does not
  maintain a `CHANGELOG.md`.
- The helper uses the repository-local `.gocache` directory. It can be removed
  when no Go process is using it.
- `make clean` removes downloaded schema files and the root generated-schema
  stamp. It does not remove `schema/version` or `sdk/version`; rerun
  `make version` afterward.
