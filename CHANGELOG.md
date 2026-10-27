# Changelog — astroneer

## v0.2.0 — unreleased

- `astroneerd`: PlayFab API is now TitleId-agnostic. The 501 stub is gone:
  the daemon answers `/Client/*` and `/Authentication/*` (plus the
  `/playfab/*` alias) locally, accepting any Host subdomain and any
  `TitleId` body value. Logins, Switch-device ID lookup, Photon auth
  token, entity token, and title/user/account reads return shaped
  PlayFab envelopes with deterministic stateless identity; unknown
  operations succeed with empty data and are logged for follow-up.
  `ASTRONEER_PLAYFAB_URL` now selects shim reverse-proxy mode for use
  after TitleId recovery.

## v0.1.0 — unreleased

- `astroneerd`: edge-service skeleton. Health (`/healthz`) and readiness
  (`/readyz`, reporting NEX and PlayFab backend state), env-driven config, and
  a PlayFab middleware stub that answers 501 until the PlayFab tenant exists
  (TitleId still inside the encrypted pak) and reverse-proxies to the shim
  after. NEX title spec delivered for serial switch-nex registration.
