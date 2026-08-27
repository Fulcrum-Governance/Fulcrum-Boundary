# Maintainers

## Current maintainer

- [Anthony Diefenbach](https://github.com/Dewars30), founder and lead maintainer

Fulcrum Boundary currently uses a single-maintainer model. The lead maintainer
owns release decisions, security coordination, public claims, adapter maturity,
and final merge authority. Community input is welcome through issues, pull
requests, and GitHub Discussions.

## Decision process

- Routine fixes and documentation changes are decided through pull-request
  review and the repository's required checks.
- New adapters, extension points, release claims, and public-positioning changes
  should begin with an issue describing the proposed scope.
- Security reports use the private process in [SECURITY.md](./SECURITY.md).
- A change does not alter release truth until its tests, evidence, and public
  documentation pass `make release-check`.

## Adding maintainers

Maintainer access is earned through sustained, high-quality contributions,
care with the routed-only safety boundary, and demonstrated ability to review
claims and release evidence. There is no fixed contribution count or schedule.

See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution scope and local gates.
