// Command astroneerd is OpenPak's ASTRONEER edge service for the Switch build
// (0100E63013E60000, UE4 "Astro") when a console is redirected to OpenPak.
//
// The game talks to two backends (see docs/protocol.md):
//
//	NEX (PRUDP) — matchmaking, NAT traversal, ranking: served by
//	platforms/switch/games/switch-nex once the title is registered there
//	(see docs/nex-title-spec.md). This daemon carries no NEX transport;
//	it only reports the configured NEX station URL on /readyz.
//	PlayFab UE4 Client API (<TitleId>.playfabapi.com) — identity and
//	backend calls: served locally and TitleId-agnostically, since the
//	real TitleId is still inside the game's encrypted pak. Any Host
//	subdomain and any TitleId field are accepted; both are only logged
//	(the capture for TitleId recovery). Point ASTRONEER_PLAYFAB_URL at
//	the shared shim after the TitleId is recovered and the shim carries
//	this title's tenant.
//
// The contract was read out of the Switch executable; see docs/protocol.md.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

// Config is every knob, each settable by flag or environment.
type Config struct {
	Addr       string // plain HTTP listen address (behind the TLS terminator)
	HealthAddr string // health listen address
	NexBase    string // switch-nex NEX port base for this title (auth=base, secure=base+1)
	NexHost    string // switch-nex host the console is told to dial
	PlayFabURL string // playfab shim base URL (empty = tenant not configured yet)
	TitleID    string // Switch title ID this service answers for
}

func main() {
	cfg := loadFlags()
	s := &server{cfg: cfg, playfab: newPlayFabStub(cfg)}

	go func() {
		h := http.NewServeMux()
		h.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
		log.Printf("[astroneer] health on %s", cfg.HealthAddr)
		log.Fatal(http.ListenAndServe(cfg.HealthAddr, h))
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	mux.HandleFunc("/readyz", s.readyz)
	// The TLS terminator forwards the game's native PlayFab paths; the
	// /playfab/ prefix is this daemon's historic alias for the same API.
	mux.Handle("/playfab/", s.playfab)
	mux.Handle("/Client/", s.playfab)
	mux.Handle("/Authentication/", s.playfab)
	log.Printf("[astroneer] serving HTTP on %s (title %s, nex %s:%s)", cfg.Addr, cfg.TitleID, cfg.NexHost, cfg.NexBase)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

type server struct {
	cfg     *Config
	playfab *playfabStub
}

// readyz reports whether the game's two middleware backends are configured.
// NEX is ready when switch-nex serves this title's port base; PlayFab is
// served locally and TitleId-agnostically unless ASTRONEER_PLAYFAB_URL
// points at the shared shim (post-TitleId-recovery).
func (s *server) readyz(w http.ResponseWriter, _ *http.Request) {
	nex := "unregistered"
	if s.cfg.NexHost != "" && s.cfg.NexBase != "" {
		nex = fmt.Sprintf("switch-nex %s:%s (auth=base secure=base+1)", s.cfg.NexHost, s.cfg.NexBase)
	}
	pf := "local titleid-agnostic"
	if s.playfab.url != "" {
		pf = "proxy " + s.playfab.url
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "title=%s\nnex=%s\nplayfab=%s\n", s.cfg.TitleID, nex, pf)
}
