# Security policy

## Supported versions

The latest minor release of Arandu Playwright receives security fixes. Older minor
releases do not: a published Go module version is immutable, so a fix is a new
release rather than a moved tag.

## Reporting a vulnerability

Report it privately, through GitHub's advisory form:

<https://github.com/hyz-is/arandu-playwright/security/advisories/new>

Do not open a public issue and do not describe the problem in a pull request.
A report that arrives in public is a report every reader of this repository can
act on before there is a release to upgrade to.

Include what you need to reproduce it: the version, the engine, and the call
that triggers it.

Expect an acknowledgement within a few days. If the report is confirmed, the
fix, the release and the advisory are published together, and you are credited
unless you ask not to be.

## What is in scope

This is test tooling, and what it can do wrong is mostly on the machine that
runs the tests. In particular:

- `Launch` downloading or executing anything other than the driver and the
  browsers already in the user's cache;
- `Install` writing outside the driver and browser directories, or into the
  project tree;
- one driver seeing another's cookies, storage or cache;
- a driver reaching a URL the test did not navigate to.

## What is not

- A vulnerability in Playwright, playwright-go or a browser. Report it
  upstream; a release here follows the fixed version.
- What an application's journeys do with the pages they open.
