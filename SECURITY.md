# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's
["Report a vulnerability"](../../security/advisories/new) form on this
repository's Security tab. Do not open a public issue for security problems.

Include the version (`/api/version`), how you run Mizu (Docker, bare binary,
behind a proxy), and steps to reproduce. You can expect an acknowledgement
within a week.

## Things to know when deploying

- Until a dashboard password is set, the UI and API are open to anyone who can
  reach the port. Set one on first launch, or put Mizu behind an auth proxy and
  make sure nothing else can reach it.
- Sonarr and artwork-provider API keys are stored in plaintext in the SQLite
  database, and the config backup export includes them. Treat the `/config`
  volume and any backup file as secrets.
- `/api/health`, `/api/version` and `/metrics` are public by design.
