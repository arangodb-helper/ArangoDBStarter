# Maintainer Instructions

## Running tests

A subset of all tests are frequently run in Travis.

To run the entire test set, run:

```bash
make run-tests
```

## Preparing a release

To prepare for a release, do the following:

- Update CHANGELOG.md. Update the first title (master -> new version). Commit it.
- Make sure all tests are OK.

## Building a release

On Linux, make sure that neither `.gobuild/tmp` nor `bin` contains any
files which are owned by `root`. For example, a `chmod -R` with your
user account is enough. This does not seem to be necessary on OSX.

To make a release you must have:

- A github access token in `~/.arangodb/github-token` that has read/write access
  for this repository.
- On OS/X, set an environment variable:
  `export MANIFESTAUTH=--username=<your-docker-hub-account> --password=<your-docker-hub-password>`
- Push permission for the current docker account (`docker login <your-docker-hub-account>`)
  for the `arangodb` docker hub namespace.
- The latest checked out `master` branch of this repository.

```bash
make release-patch
# or
make release-minor
# or
make release-major
```

If successful, a new version will be:

- Build for Mac, Windows & Linux (all amd64).
- Tagged in github
- Uploaded as github release
- Pushed as docker image to docker hub
- `./VERSION` will be updated to a `+git` version (after the release process)

If the release process fails, it may leave:

- `./VERSION` uncommitted. To resolve, checkout `master` or edit it to
  the original value and commit to master.
- A git tag named `<major>.<minor>.<patch>` in your repository.
  To resolve remove it using `git tag -d ...`.
- A git tag named `<major>.<minor>.<patch>` in this repository in github.
  To resolve remove it manually.

## Completing after a release

After the release has been build (which includes publication) the following
has to be done:

- Update CHANGELOG.md. Add a new first title (released version -> master). Commit it.
