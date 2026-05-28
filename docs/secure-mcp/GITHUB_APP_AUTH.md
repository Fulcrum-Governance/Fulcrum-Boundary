# Secure GitHub GitHub App Auth

Secure GitHub live conformance uses operator-owned GitHub App credentials. The
credential path is opt-in and is separate from the default fixture demo.

## Source Contract

Boundary follows the current GitHub App REST authentication flow:

- Generate a short-lived GitHub App JWT signed with `RS256`.
- Exchange that JWT for an installation access token with
  `POST /app/installations/{installation_id}/access_tokens`.
- Use the installation token only for the conformance run.
- Send `X-GitHub-Api-Version: 2026-03-10` on GitHub REST requests.

References:

- [Generating a JSON Web Token for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)
- [Generating an installation access token for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)
- [GitHub REST API versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions?apiVersion=2026-03-10)

## Required Environment

Live conformance is skipped unless this variable is set:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true
```

When enabled, these variables are required:

```bash
BOUNDARY_GITHUB_APP_ID
BOUNDARY_GITHUB_INSTALLATION_ID
BOUNDARY_GITHUB_PRIVATE_KEY_PATH
BOUNDARY_GITHUB_OWNER
BOUNDARY_GITHUB_REPO
BOUNDARY_GITHUB_ISSUE_NUMBER
```

Optional:

```bash
BOUNDARY_GITHUB_API_BASE_URL
BOUNDARY_GITHUB_TRANSCRIPT_DIR
BOUNDARY_GITHUB_TRANSCRIPT
```

## Credential Handling

- Private keys are read at runtime from the configured path.
- The configured private key path is not written to conformance transcripts.
- Installation tokens are cached in memory only until near expiry.
- Errors redact GitHub token-like response data.
- Sanitized transcripts include hashes and booleans, not issue bodies,
  installation tokens, JWTs, private keys, or private key paths.

Do not commit `.pem` files, installation tokens, raw HTTP logs, or raw
transcripts.

