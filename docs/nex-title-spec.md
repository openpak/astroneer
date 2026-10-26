# NEX title spec — ASTRONEER (for switch-nex registration)

Handoff for the serial switch-nex registry integration. This repo is not
edited into switch-nex here; the registrar copies this spec into
`cmd/switch-nexd/main.go` (registry) and `titles/<slug>/` once the TODO-KEY
items resolve.

Wire facts sourced from `scratch/decompile-astroneer/RECON.md` and
`scratch/decompile-astroneer/findings.json` (see [protocol.md](protocol.md)).

## Title

| Field | Value |
| --- | --- |
| Slug | `astroneer` |
| Title ID | `0100E63013E60000` |
| Display version | `1.1.42.0` |
| Engine | UE4 (project `Astro`) |
| NEX version | ~4.6.5 UNCONFIRMED (no version literal in binary; confirm via `nn::nex::VersionInfo::V1..V4` in Ghidra) |

## Port-base request

- Requested base: `22200` (auth=22200, secure=22201, healthz=22202).
- Rationale: registry in `switch-nex/cmd/switch-nexd/main.go` already assigns
  22110 (mario-party-superstars), 22120 reserved (MHGU), 22130
  (mario-golf-super-rush), 22140 (arms), 22150 (mario-kart-8-deluxe), 22160
  (super-mario-bros-35), 22170 (mario-tennis-aces), 22180
  (animal-crossing-new-horizons), 22190 (luigis-mansion-3). 22200 is the next
  free base in the +10 stride.

## Protocol set (client-verified present)

Authentication, SecureConnection, MatchMaking + MatchMakingExt,
MatchmakeExtension, NATTraversal + NATTraversalRelay, Ranking2,
TicketGranting, Notification (+NintendoNotificationEvent), Dns, Http,
NATCheck, Health, Monitoring.

Explicitly NOT in the client: DataStore, Utility, Messaging, Subscriber —
the title entry must not enable them.

## Outstanding keys (TODO-KEY)

| Item | Status |
| --- | --- |
| NEX access key (Kerberos password for this title) | TODO-KEY: not in binary strings; recover via Ghidra on `main.dec` (StationURL / game-server-ID path) |
| NEX game server ID | TODO-KEY: runtime/config value, same recovery path |
| NEX server URL / StationURL | TODO-KEY: same recovery path |
| NEX version confirmation (~4.6.5?) | TODO-KEY: confirm via `VersionInfo::V1..V4` before pinning protocol behaviour |
| PlayFab TitleId | TODO-KEY: inside AES-encrypted `Astro-Switch.pak` cooked config; needs pak AES key (Ghidra) then `DefaultEngine.ini`; registers a tenant in the playfab shim (`PLAYFAB_ASTRONEER_TITLE` pattern) |
| Photon AppIds (`PhotonRealtimeAppId`, `PhotonChatAppId`, `PhotonTurnbasedAppId`) | TODO-KEY: PlayFab-mediated names only; realtime transport path (UE4 replication vs NEX) still TBD via Ghidra/CPU trace |

## Registrar checklist

1. Confirm NEX version via Ghidra (`VersionInfo::V1..V4`).
2. Recover StationURL / game server ID / access key; fill TODO-KEY rows.
3. Add registry entry at base 22200 with the protocol set above (no DataStore).
4. Add `titles/astroneer/` README shadowing the mario-kart-8-deluxe layout.
5. Decrypt pak → `DefaultEngine.ini` → PlayFab TitleId → shim tenant; set
   `ASTRONEER_PLAYFAB_URL` on this repo's daemon.
