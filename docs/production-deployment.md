# Production deployment

Production releases are managed by `.github/workflows/deploy-production.yml`.
Do not rebuild or recreate the storefront manually during normal development.

## Triggers

- A push to `main`.
- A push to `agent/harden-payment-guest-auth-security` while that branch is the production integration branch.
- A manual `workflow_dispatch` run in GitHub Actions.

## Deployment contract

The `dovelora-production` self-hosted runner invokes the root-owned
`/usr/local/sbin/dovelora-deploy` command with the exact Git commit SHA. The
script fetches and archives that commit, builds one immutable Docker image,
updates `/Project/deployments/dujiao/deploy.env`, and recreates only `dujiao`
with `--no-deps`.

Local and public HTTP health checks must pass before the release is accepted.
On failure, the image pointer and `dujiao` container are restored to the
previous image. The `epusdt` container ID and start time are compared before
and after every deployment; the workflow never recreates the payment service.

## Manual recovery

Use GitHub Actions to rerun a known-good commit. The deployment script accepts
only a full 40-character commit SHA and can deploy any commit that is available
from the repository's `origin` remote.
