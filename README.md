# ASTRONEER

OpenPak's ASTRONEER Server.

| | |
| --- | --- |
| Game | ASTRONEER |
| Network | Nintendo Switch |
| Title ID | `0100E63013E60000` |
| Engine | Unreal Engine 4 (project `Astro`) |
| Licence | AGPL-3.0-only |

## What it serves

| Host | What | Served by |
| --- | --- | --- |
| NEX (PRUDP) | Matchmaking, NAT traversal, ranking | [`switch-nex`](https://github.com/openpak/switch-nex) (title registration pending, see [NEX title spec](docs/nex-title-spec.md)) |
| `<TitleId>.playfabapi.com` | PlayFab UE4 Client API (identity, backend calls) | this repo (`astroneerd`, TitleId-agnostic; [`playfab`](https://github.com/openpak/playfab) proxy after TitleId recovery) |
| local | Edge service: health, readiness, PlayFab API | this repo (`astroneerd`) |

## Run

```sh
GOWORK=off go build -o astroneerd ./cmd/astroneerd
./astroneerd -addr :23560 -health-addr :23561
```

| Flag / env | Meaning |
| --- | --- |
| `-addr` / `ASTRONEER_HTTP_ADDR` | plain HTTP listen address (`:23560`) |
| `-health-addr` / `ASTRONEER_HEALTH_ADDR` | health listen address (`:23561`) |
| `-nex-base` / `ASTRONEER_NEX_BASE` | switch-nex NEX port base for this title (`22200`) |
| `-nex-host` / `ASTRONEER_NEX_HOST` | switch-nex host the console dials |
| `-playfab-url` / `ASTRONEER_PLAYFAB_URL` | playfab shim base URL (empty serves PlayFab locally, TitleId-agnostically) |
| `-title-id` / `ASTRONEER_TITLE_ID` | Switch title ID (`0100E63013E60000`) |

## Docs

- [Protocol](docs/protocol.md)
- [NEX title spec](docs/nex-title-spec.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
