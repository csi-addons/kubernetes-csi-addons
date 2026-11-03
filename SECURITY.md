# Security Policy

## Supported Versions

Security fixes are provided for the latest released version of
Kubernetes-CSI-Addons. Older releases are not maintained unless explicitly
announced in the [release notes](https://github.com/csi-addons/kubernetes-csi-addons/releases).

| Version        | Supported          |
| -------------- | ------------------ |
| latest release | :white_check_mark: |
| older releases | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, pull requests or the mailing list.**

Instead, report them privately using GitHub's
[private vulnerability reporting](https://github.com/csi-addons/kubernetes-csi-addons/security/advisories/new)
for this repository.

Please include as much of the following information as possible to help us
triage the report:

- the type of issue (for example privilege escalation, information disclosure,
  denial of service),
- the affected component (controller, sidecar, CRD, deployment manifests) and
  version or commit,
- step-by-step instructions to reproduce the issue,
- proof-of-concept or exploit code, if available,
- the impact of the issue, including how an attacker might exploit it.

## Disclosure Process

1. The report is acknowledged within 5 working days.
2. The maintainers confirm the issue, determine the affected versions, and
   discuss a fix with the reporter in a private security advisory.
3. A fix is prepared and reviewed privately, and a new release is made.
4. A security advisory (and CVE, where applicable) is published together with
   the release. The reporter is credited unless they prefer to remain
   anonymous.

We ask reporters to follow coordinated disclosure and to allow up to 90 days
from the initial report for a fix to be released before any public disclosure.
