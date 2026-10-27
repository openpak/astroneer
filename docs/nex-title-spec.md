# NEX title spec — ASTRONEER (for switch-nex registration)

Handoff for the serial switch-nex registry integration. This repo is not
edited into switch-nex here; the registrar copies this spec into
`cmd/switch-nexd/main.go` (registry). Patched-client profile (see
`tools/astro-key/`): the production key is unrecoverable statically, so
consoles run the `openpak_astro_key` patch and the title serves them with
an empty access key.

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

- Requested base: `23180` (auth=23180, secure=23181, healthz=23182) — next
  free in the 231xx third-party batch after lethal-league-blaze (23170);
  verified free of the registry on 2026-10-27. (First pick 22200 was
  already bound on the production host by a non-NEX title outside the
  registry survey, the same collision class as Gang Beasts' 22210.)

## Protocol set (client-verified present)

Authentication, SecureConnection, MatchMaking + MatchMakingExt,
MatchmakeExtension, NATTraversal + NATTraversalRelay, Ranking2,
TicketGranting, Notification (+NintendoNotificationEvent), Dns, Http,
NATCheck, Health, Monitoring.

Explicitly NOT in the client: DataStore, Utility, Messaging, Subscriber —
the title entry must not enable them.

## Keys (patched-client profile, TODO-KEY closed 2026-10-27)

Native analysis (update main, build `35DD08…`) proved the production pair
unrecoverable statically: both values are `FConfig::GetString` reads from
`[/Script/SwitchRuntimeSettings]` (`GameServerId` at code ref `0xfe3d20`,
`GameServerAccessKey` at `0x10292f8`), with no default literals — they live
only in the AES-encrypted pak, and no UE4 pak/AES key strings exist in the
binary. Consoles therefore run the `openpak_astro_key` patch
(`tools/astro-key/`, owner-approved 2026-10-27):

| Item | Value |
| --- | --- |
| NEX access key | `""` (empty). The patch renames the ini key so the read misses and the client takes its guarded empty path. Server runs the same empty key (`risk-of-rain-returns` precedent). |
| NEX game server ID | `0xA57` (routing only). The patch pins the login SNI to `ga57-lp1.s.n.srv.nintendo.net`; the real ini ID is never validated server-side and flows through untouched. |
| NEX version | ~4.6.5 still UNCONFIRMED (no banner in binary); entry uses the pre-5.19 legacy Pia shape like the other unmeasured NEX 4.6.x peers. |
| PlayFab TitleId | Still TODO-KEY (AES pak). No client patch needed: `*.playfabapi.com` already routes to us and the shim answers TitleId-agnostically. |
| Photon AppIds | Still TODO-KEY; no Photon client linked, realtime path TBD at runtime. |

## Registry entry (landed in switch-nex)

```go
{"astroneer", 23180, "", func(e *protocols.Env) {
    e.StationScheme = "prudps"
    e.Pia = protocols.LegacyPia()
    e.Match = protocols.NewMatchmakingStore()
    e.FinalizeCreateSession = true
    e.JoinEchoSession = true
    e.JoinRecapParticipate = true
}}
```

Matchmaking set only; Ranking2/Notification/NAT-Traversal stay unregistered
until the first boot's NotImplemented log. No DataStore/Utility/Messaging/
Subscriber (not in the client). Provenance:
`switch-nex/docs/astroneer.md`.
