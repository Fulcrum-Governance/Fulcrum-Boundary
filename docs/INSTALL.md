# Install

Fulcrum Boundary ships the `boundary` CLI from the Go module
`github.com/fulcrum-governance/fulcrum-boundary`. The binary name remains
`boundary`.

## Go Install

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@main
boundary selftest
```

`@main` follows the current public default branch while the first post-rename
release tag is pending. After a post-rename tag is cut, this command can move
back to `@latest`.

## From Source

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
```

Source checkouts also include Make targets for the no-credential release path:

```bash
make selftest
make demo-github
make release-check
```

## First Useful Commands

Run the local release smoke test:

```bash
boundary selftest
```

Run the fixture-only GitHub lethal-trifecta demo:

```bash
boundary demo github-lethal-trifecta
```

The demo uses fixture data and does not require live GitHub credentials or make
upstream GitHub mutations.

## Uninstall

Remove the binary installed by Go:

```bash
rm "$(go env GOPATH)/bin/boundary"
```

If you used `boundary install` to rewrite an MCP client config, restore through
the install receipt created at install time:

```bash
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json
```

Use `--dry-run` first to inspect the planned restore without mutating local MCP
client config files.

## Optional Packaging Placeholder

Homebrew formula distribution is planned. Do not use Homebrew commands in public
install docs until an actual tap exists.
