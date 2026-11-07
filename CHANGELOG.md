# Changelog

All notable changes to NodeMon are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/). See `docs/development.md` for how a release is made.

NodeMon is an independent project, not affiliated with Netdata, Inc.

## [Unreleased]

### Added
- Versions on screen: the dashboard's version (account menu and login page) and the version of the connector each
  computer runs (nodes table and computer page, with an "old version" badge when a connector is older than the
  dashboard). The connector reports its version in a new `cv` field.
- Per-program usage on the computer detail page (Linux only, from Netdata's apps charts): which programs used the
  most CPU and memory, a chart of their CPU over the chosen range, average and peak per program, an estimate of the
  CPU time each used, and a warning for programs whose CPU jumped far above their usual level. Computers that
  cannot report it (macOS, Windows, or an older connector) show an explanation instead.
- The connector sends the top 4 CPU users and top 3 memory users as one short text field `ap`; the database rules
  accept it. This adds roughly 2 to 4 MB per month for a Linux computer with the default 3-minute uploads.

- Automatic deployment: a GitHub Actions workflow publishes the dashboard and the security rules whenever `main`
  changes, using a service account key and the allowed accounts stored as repository secrets. It does nothing until
  those secrets exist. `firebase/deploy.mjs` has a matching `--ci` mode (no prompts, no interactive login).

### Changed
- Visual refresh: new color tokens and typography, rounded cards with soft shadows, gradient accents, pill controls,
  usage bars that turn amber above 80% and red above 90%, gradient chart fills, a frosted top bar, a redesigned login
  page and add-computer window, and refined light and dark themes.

## [0.1.0] - 2026-11-06

First release. NodeMon shows the resource usage of one or many computers on a dashboard that runs on the free
Firebase Spark plan, using the free Netdata Agent as the data source.

### Added
- Dashboard (React, Vite, TypeScript) styled after Netdata Cloud: Google sign-in, overview of all computers, node
  table with sorting and filtering, per-computer detail with charts for 15 minutes to 30 days, light and dark themes,
  and a demo build with fake data.
- Firebase backend without any server: Realtime Database security rules generated for the allowed Google accounts,
  one-time pairing codes to register computers, and `firebase/deploy.mjs`, a one-shot deploy script.
- Connector (Go, standard library only) that reads the local Netdata Agent and uploads to Firebase. Measures every
  10 s, averages locally and uploads every 3 minutes in one request, with a retry queue and back-off.
- Data-saving measures: batched uploads, rounded values, silent writes, kept-alive HTTP/2 connections and an
  experimental `--compress` gzip mode. About 17.5 MB per computer per month with the defaults (see
  `docs/data-and-usage.md`).
- Per-computer upload total (counted at the network socket) and uptime on the nodes table and detail page.
- "Add computer" window: computer name, operating system and CPU type selection, download, run and autostart
  commands for Linux, macOS and Windows, and a hint for finding out the CPU type.
- Delete a computer from the nodes table or its detail page; its data and access are removed.
- Connector programs for Linux (amd64, arm64, arm), macOS (arm64, amd64) and Windows (amd64), published as ZIP files
  on GitHub Releases with `SHA256SUMS`.
- GPL-3.0-or-later license, generated third-party license notices, and statements that NodeMon is not affiliated with
  Netdata, Inc.
- CI: formatting, vet, staticcheck, race tests, cross-compilation, TypeScript checks, repository consistency checks
  and the Firebase emulator test suite.

### Changed
- The accent color is no longer Netdata's green.

### Removed
- The self-hosted server option (Go server, Docker files). The last commit that contains it is `3dd3617`.

[Unreleased]: https://github.com/CHOICHI-HWAN/NodeMon/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/CHOICHI-HWAN/NodeMon/releases/tag/v0.1.0
