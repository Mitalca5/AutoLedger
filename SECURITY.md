# Security policy

## Supported versions

Fixes are released on the latest version only. Update to the latest release (`ghcr.io/rem7474/autoledger`) before reporting.

## Reporting a vulnerability

Please do not open a public issue. Use GitHub's private reporting: **Security → Report a vulnerability** on the repository ([direct link](https://github.com/Rem7474/AutoLedger/security/advisories/new)).

Include the affected version, the steps to reproduce and the impact you see. You will get an answer as soon as possible, and the report stays private until a fix is released.

## Scope

In scope: authentication and sessions, API tokens, access control between users and vehicles, stored credentials, document storage, the Docker images.

Out of scope: findings that need access to the host or to the database, missing hardening on a deployment that does not follow the reverse-proxy guidance of the README, and reports produced by automatic scanners without a demonstrated impact.
