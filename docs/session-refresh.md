# Session renewal

Français : [session-refresh.fr.md](session-refresh.fr.md).

The browser stores access and refresh tokens in HttpOnly cookies. Requests that receive an authentication error can renew the session and retry once, preserving their body and idempotency key.

Tabs on the same origin coordinate renewal through Web Locks in supported secure contexts (HTTPS or localhost). A tab that waited for another successful renewal reuses the updated cookies. Only a random coordination marker is stored in local storage; it contains no token or account details. When local storage is unavailable, the lock still serializes renewals.

Browsers without Web Locks retain coordination within each tab only. Use HTTPS for deployments accessed through a hostname or LAN address. Server-side refresh-token reuse detection remains enabled.
