# Changelog — astroneer

## v0.1.0 — unreleased

- `astroneerd`: edge-service skeleton. Health (`/healthz`) and readiness
  (`/readyz`, reporting NEX and PlayFab backend state), env-driven config, and
  a PlayFab middleware stub that answers 501 until the PlayFab tenant exists
  (TitleId still inside the encrypted pak) and reverse-proxies to the shim
  after. NEX title spec delivered for serial switch-nex registration.
