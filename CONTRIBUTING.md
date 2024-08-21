# Contribution Guidelines

Great, you're looking how to contribute to the Kubernetes CSI-Addons project!
Many thanks for your interest :smiley: :tada:

The guidelines in this document try to be helpful and assist you while working
with others on this project. Its intention is to make contributions as easy and
straight forward as possible.

## Contributing changes to the project

The Kubernetes CSI-Addons project uses a [GitHub workflow][github_pr] for
contributions. That means, changes are sent as Pull-Requests from a forked
repository.

## Reviewing of Pull-Requests

All Pull-Requests are required to be reviewed by at least two others that
regularly participate in the project. There are two GitHub teams that contain
members who can approve changes:

- @csi-addons/kubernetes-csi-addons-contributors: regular contributors,
  sending Pull-Requests, designing new features
- @csi-addons/kubernetes-csi-addons-reviewers: contributors to the general
  CSI-Addons project, sharing expertise and domain knowledge

For changes that are related to the integration with other components or affect
the user interface (Pull-Requests with the `api` label), an approval from
someone in the @csi-addons/kubernetes-csi-addons-reviewers team is required.

## Merging Pull-Requests

After a Pull-Request has been reviewed and approved, it will get merged
automatically by the Mergify bot (@mergifyio).

[github_pr]: https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request
