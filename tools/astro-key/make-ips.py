#!/usr/bin/env python3
"""make-ips.py <main.nso> [out-dir]: write the Astroneer NEX key/routing patch.

Astroneer reads its NEX credentials from cooked config
([/Script/SwitchRuntimeSettings] GameServerId / GameServerAccessKey in
DefaultEngine.ini) inside the AES-encrypted Astro-Switch.pak, so the
production key is not recoverable from the executable. This patch
re-aims the client at OpenPak's NEX server with two read-only data
edits and no code change:

1. The wide ini key name "GameServerAccessKey" becomes
   "XameServerAccessKey", which exists nowhere, so the FConfig read
   misses and the client proceeds with an EMPTY access key (the miss
   path is null-guarded: empty length selects a NUL default, the
   narrow conversion yields "", frees are cbz-guarded). OpenPak's
   astroneer title runs the same empty key, the documented
   risk-of-rain-returns shape: CONNECT signatures go unverified and
   the server never signs anything the client checks.

2. The NEX login SNI format "g%08x-%%.s.n.srv.nintendo.net" becomes the
   fixed "ga57-lp1.s.n.srv.nintendo.net" (ID 0xA57, exact-fit 30
   bytes). The game server ID itself is never validated by
   switch-nex -- it only selects the server via DNS -- so the real,
   still-unknown ini value flows through untouched and only the
   routing name is pinned. The fixed string carries no format verbs,
   so the extra id argument is ignored by both format stages.

Everything is found by content, not by hard-coded address: the script
refuses unless each source string occurs exactly once in the input.
Offsets in the IPS include the NSO header, the convention Atmosphere
and the emulators share. Offsets exceed 24 bits, so the file is IPS32.

Usage: make-ips.py <main.nso> [out-dir]   (writes <BUILDID32>.ips)
"""
import struct
import sys
import os

KEY_NAME = "GameServerAccessKey".encode("utf-16-le") + b"\x00\x00"
KEY_NAME_NEW_FIRST = "X".encode("utf-16-le")  # GameServerAccessKey -> XameServerAccessKey
SNI_FMT = b"g%08x-%%.s.n.srv.nintendo.net\x00"
SNI_NEW = b"ga57-lp1.s.n.srv.nintendo.net\x00"  # ID 0xA57
assert len(SNI_NEW) == len(SNI_FMT)


def find_once(data, needle, what):
    first = data.find(needle)
    if first < 0:
        raise SystemExit(f"refusing: {what} not found")
    if data.find(needle, first + 1) >= 0:
        raise SystemExit(f"refusing: {what} occurs more than once")
    return first


def main():
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        raise SystemExit("usage: make-ips.py <main.nso> [out-dir]")
    src = sys.argv[1]
    outdir = sys.argv[2] if len(sys.argv) > 2 else os.path.dirname(os.path.abspath(__file__))
    data = open(src, "rb").read()
    if data[:4] != b"NSO0":
        raise SystemExit("refusing: not a decompressed NSO (NSO0 magic missing)")
    build = data[0x40:0x50].hex().upper()

    key_off = find_once(data, KEY_NAME, "wide GameServerAccessKey")
    sni_off = find_once(data, SNI_FMT, "SNI format")

    recs = [(key_off, KEY_NAME_NEW_FIRST), (sni_off, SNI_NEW)]
    ips = bytearray(b"IPS32")
    for off, payload in recs:
        ips += struct.pack(">IH", off, len(payload)) + payload
    ips += b"EEOF"

    os.makedirs(outdir, exist_ok=True)
    out = os.path.join(outdir, build + ".ips")
    open(out, "wb").write(bytes(ips))
    print(f"build {build}")
    print(f"  key name -> XameServerAccessKey at fileoff 0x{key_off:x} ({len(KEY_NAME_NEW_FIRST)} bytes)")
    print(f"  SNI fmt  -> ga57-lp1... at fileoff 0x{sni_off:x} ({len(SNI_NEW)} bytes)")
    print(f"wrote {out} ({len(ips)} bytes)")


if __name__ == "__main__":
    main()
