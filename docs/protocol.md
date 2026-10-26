# ASTRONEER (Switch) — network protocol

Wire facts read from the Switch executable (`0100E63013E60000`, UE4 project
`Astro`). Sources, in the workspace:

- `scratch/decompile-astroneer/RECON.md`
- `scratch/decompile-astroneer/findings.json`

Keep this file in step with `cmd/astroneerd`. Facts here are protocol-level
only; the recon files stay private and are not copied into this repo.

## Identity

- Title ID: `0100E63013E60000` (findings.json `title_id`).
- Display version: `1.1.42.0` (RECON.md "Source dump").
- Engine: Unreal Engine 4, `OnlineSubsystemSwitch` + `OnlineSubsystemNull`
  (RECON.md "Engine / middleware verdict").

## NEX (statically linked client)

The `main` executable carries a statically linked NEX client (`nn::nex::*`,
~6800 symbols): StationURL, PRUDP endpoint/stream/socket, Kerberos auth,
NAT traversal + relay (RECON.md "Engine / middleware verdict",
findings.json `middleware.nex`).

Client protocols present:

- Authentication, SecureConnection, MatchMaking (+Ext), MatchmakeExtension,
  NATTraversal (+Relay), Ranking2, TicketGranting,
  Notification (+NintendoNotificationEvent), Dns, Http, NATCheck, Health,
  Monitoring (findings.json `nex_protocols`).

Explicitly absent: DataStore, Utility, Messaging, Subscriber
(RECON.md "Engine / middleware verdict", findings.json
`middleware.nex.datastore=false`).

NEX version string was NOT found in the binary; the prior claim of 4.6.5 is
UNCONFIRMED and needs Ghidra follow-up on `nn::nex::VersionInfo::V1..V4`
(RECON.md "Engine / middleware verdict", findings.json
`middleware.nex.version_confirmed=false`).

The NEX server URL, game server ID, and PRUDP key are runtime/config values —
no game-specific host or key literals in the binary (RECON.md
"Engine / middleware verdict"). See [nex-title-spec.md](nex-title-spec.md)
for the registration handoff.

## PlayFab (UE4 plugin)

- PlayFab UE4 plugin with `AstroPlayFab*` game classes; Client API calls
  include `/Client/GetPlayFabIDsFrom*` and
  `/Client/GetPhotonAuthenticationToken` (RECON.md "Engine / middleware
  verdict", findings.json `middleware.playfab`).
- Base URL `https://playfabapi.com`; the TitleId subdomain is set at runtime
  and no TitleId literal is in the binary — it lives in cooked config inside
  the AES-encrypted pak (RECON.md "Engine / middleware verdict" and
  "RomFS / pak", findings.json `middleware.playfab.title_id_literal=false`).
- The shared shim (`platforms/switch/games/playfab`) serves tenants per
  title: one PlayFab TitleId plus the Switch title ID it is scoped to
  (the `PLAYFAB_DOOM_TITLE` / `PLAYFAB_DOOM_APP` pattern in the shim's
  `titles.go`). ASTRONEER joins the same way once its TitleId is recovered.

## What is not here

- No Photon client SDK is linked (0 hits for PhotonPeer/OperationRequest/
  LoadBalancing/ExitGames); only PlayFab-mediated names
  (`PhotonRealtimeAppId`, `PhotonChatAppId`, `PhotonTurnbasedAppId`,
  `GetPhotonAuthenticationToken`). The realtime transport path is TBD —
  likely UE4 replication or NEX (RECON.md "Engine / middleware verdict",
  findings.json `middleware.photon_client_sdk=false`).
- No EOS/Steam online subsystems (RECON.md "Engine / middleware verdict").
- HTTP/TLS stack: `libcurl/7.64.1`, `nghttp2`, UE4 WebSocket driver
  (findings.json `tls_http`).
- Voice: non-Vivox `VoiceChat` (RECON.md "Engine / middleware verdict").

## astroneerd's surface

`astroneerd` is the edge service; the game backends live elsewhere:

- `GET /healthz` — `ok` (both listeners).
- `GET /readyz` — title, NEX base, and PlayFab tenant state.
- `/playfab/*` — PlayFab middleware stub: 501 until
  `ASTRONEER_PLAYFAB_URL` points at a shim carrying this title's tenant,
  then reverse-proxied to the shim's `/Client/*` API.
