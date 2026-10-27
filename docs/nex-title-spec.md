# NEX title spec — ASTRONEER (for switch-nex registration)

Handoff for the serial switch-nex registry integration. This repo is not
edited into switch-nex here; the registrar copies this spec into
`cmd/switch-nexd/main.go` (registry). Production pair (the paks are
plaintext; the earlier `tools/astro-key/` patched-client profile is
retired and its patch set superseded).

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

## Keys (production pair, TODO-KEY closed 2026-10-27)

Native analysis (update main, build `35DD08…`) showed both values are
`FConfig::GetString` reads from `[/Script/SwitchRuntimeSettings]`
(`GameServerId` at code ref `0xfe3d20`, `GameServerAccessKey` at
`0x10292f8`), with no default literals. The paks were first thought
AES-encrypted; they are fully plaintext (0 encrypted entries,
index-SHA1 verified), so the values read straight out of
`BaseSwitchEngine.ini`, identical in base v0 and update v2293760:

| Item | Value |
| --- | --- |
| NEX access key | `18dbde6c` |
| NEX game server ID | `2EE4D000` (787480576; derived SNI `g2ee4d000-lp1`) |
| NEX version | ~4.6.5 still UNCONFIRMED (no banner in binary); entry uses the pre-5.19 legacy Pia shape like the other unmeasured NEX 4.6.x peers. |
| PlayFab TitleId | `5EA1` (recovered from update `DefaultEngine.ini`; secret stays scratch-only, never published). No client patch needed: the daemon answers TitleId-agnostically. |
| Realtime | `NEXNetDriver` (NEX PRUDP, not Photon); no net encryption; MaxPlayers 8/16. |

History: the `tools/astro-key/` patched-client profile (empty key,
fixed SNI `ga57-lp1`) served before the plaintext finding; its patch
set is superseded and the `ga57-lp1` router retired.

## Registry entry (landed in switch-nex)

```go
{"astroneer", 23180, "18dbde6c", func(e *protocols.Env) {
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
