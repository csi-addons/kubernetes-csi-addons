# Continuous Integration

## GitHub Workflows

The GitHub Workflows under [`.github/workflows/][workflows] contain jobs that
are started when Pull-Requests are created or updated. Some of the jobs can
build container-images for multiple architectures. Not everyone or all
environmens wants to run the build tests for all platforms. The workflows can
be configured to select platforms that the `docker/setup-buildx-action`
supports.

For this configuration, a new Secret should be created in the GitHub
Settings of the repository. 'Normal' environment variables seem not possible.

An example of the GitHub Secret that will build the container-images on AMD64,
and both 32-bit and 64-bit Arm platforms:

- `BUILD_PLATFORMS`: `linux/amd64,linux/arm64,linux/arm/v7`

Detailed steps on creating the GitHub Secret can be found in [the GitHub
Documentation][gh_doc_secret].

In case the `BUILD_PLATFORMS` environment variable is not set, the
`docker/setup-buildx-action` action defaults to the single architecture where
the workflow is run (usually `linux/amd64`).

[workflows]: .github/workflows/
[gh_doc_secret]: https://docs.github.com/en/actions/security-guides/encrypted-secrets#creating-encrypted-secrets-for-a-repository
