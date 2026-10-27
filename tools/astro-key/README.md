# Astroneer NEX key/routing patch

> **Release build: re-aims the console client at OpenPak's NEX server with an empty access
> key and a fixed routing SNI.** Two read-only data edits, no code change, one redirected
> ini read and one fixed hostname. It is the artifact to ship (owner-approved exception,
> same class as the CTR key patch).

## The Astroneer exception to "no game patches"

OpenPak does not patch games. It changes where the console connects and puts its CA in the
console's certificate store. This patch is a documented exception, in the same class as the
CTR key patch (`openpak_ctr_key`, which swaps the key Crash Team Racing checks its login
replies against): it changes only which server the title trusts, never what the game does.

Why no console-side fix reaches it: Astroneer (UE4) reads its NEX credentials from cooked
config — `[/Script/SwitchRuntimeSettings]` `GameServerId` / `GameServerAccessKey` in
`DefaultEngine.ini` — inside the AES-encrypted `Astro-Switch.pak`. The production access
key is not in the executable at all (no literal, no default; the readers are the only two
code references to the key names). The key is the NEX Kerberos shared secret: it never
appears on the wire, so no server-side change can learn it, and every unpatched client keeps
failing against OpenPak with the real key. DNS already routes any `g*-lp1.s.n.srv.nintendo.net`
to the box and the console already trusts the OpenPak CA; the key is the one value neither
reaches.

## What the release patch does

Two IPS32 records over read-only data:

1. **Empty access key.** The wide ini key name `GameServerAccessKey` becomes
   `XameServerAccessKey` (first character only), which exists nowhere, so the `FConfig`
   read misses. The miss path is the client's own guarded empty path: zero length selects
   the NUL default, the narrow conversion yields `""`, and the frees are null-checked — no
   abort, no crash. OpenPak's `astroneer` NEX title runs the same empty key, the documented
   `risk-of-rain-returns` shape: CONNECT signatures go unverified, which costs nothing
   because the server never signs its own CONNECT-Ack and the client has nothing of ours
   to check.
2. **Fixed routing SNI.** The NEX login format `g%08x-%%.s.n.srv.nintendo.net` becomes
   `ga57-lp1.s.n.srv.nintendo.net` (ID `0xA57`, an exact-fit 30-byte overwrite). The game
   server ID is never validated by switch-nex — it only selects the server via DNS — so the
   real ini value flows through untouched and only the routing name is pinned. The fixed
   string carries no format verbs, so the id argument is ignored by both format stages.

| Build | What | Patch sites (file offsets) |
|---|---|---|
| `35DD08251AEDF2D208E594A0390157E9` | update v2293760 (v1.44.3.0), the build consoles run | key name `0x769b10a` (sole code ref `0x10292f8`); SNI `0x74b600e` (refs `0x4facbb0`, `0x4facfe8`) |
| `B707145B9C35A12A1D493F479B7F8538` | base v0 NSP | key name `0x55815c0` (sole code ref `0x7ac19c`); SNI `0x5736593` (refs `0x36969a0`, `0x3696dd8`) |

## Regenerating for a new update

```
make-ips.py <main.nso> [out-dir]
```

Nothing is hard-coded to an address. The run locates the wide key name and the SNI format by
content and refuses unless each occurs exactly once; the output name is the input's build id.
Offsets in the IPS include the NSO header, the convention Atmosphere and the emulators share.
The game has no code cave (max `.text` zero run is 15 bytes), which is why the patch is
data-only: a missed read plus a fixed hostname need no executable stub.

## Install

The file name is the build id. The loader picks the file that matches.

| Where | Path |
|---|---|
| Switch (Atmosphere) | `sd:/atmosphere/exefs_patches/openpak_astro_key/<build id>.ips` |
| Harbor (Eden-based) | `<user>/load/0100E63013E60000/openpak-astro-key/exefs/<build id>.ips` |

Delivery to players goes the same way as the other game patches (the OpenPak Switch Patcher
ships the set; Harbor picks it up as a built-in), with both builds above.

Server side: `switch-nex` title `astroneer` (empty access key, routed SNI `ga57-lp1`); the
PlayFab endpoint answers TitleId-agnostically, so the PlayFab path needs no client change.
