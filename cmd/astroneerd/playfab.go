package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// playfabStub is the middleware client for the game's PlayFab UE4 Client API
// calls (<TitleId>.playfabapi.com). The shared shim at
// platforms/switch/games/playfab serves tenants per title: each tenant is a
// PlayFab TitleId plus the Switch title ID it is scoped to (the
// PLAYFAB_DOOM_TITLE / PLAYFAB_DOOM_APP pattern in the shim's titles.go).
// ASTRONEER's TitleId lives in cooked config inside the AES-encrypted pak and
// is not recovered yet, so until ASTRONEER_PLAYFAB_URL points at a shim that
// carries this title's tenant, the stub answers 501 and logs what the client
// asked for — a capture shows what to implement next.
type playfabStub struct {
	url    string
	proxy  *httputil.ReverseProxy
	target *url.URL
}

func newPlayFabStub(cfg *Config) *playfabStub {
	s := &playfabStub{url: cfg.PlayFabURL}
	if cfg.PlayFabURL == "" {
		return s
	}
	u, err := url.Parse(cfg.PlayFabURL)
	if err != nil {
		log.Printf("[astroneer] !! bad playfab url %q: %v (stub stays 501)", cfg.PlayFabURL, err)
		s.url = ""
		return s
	}
	s.target = u
	s.proxy = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.URL.Path = "/Client" + r.In.URL.Path[len("/playfab"):]
			r.Out.URL.RawQuery = r.In.URL.RawQuery
		},
	}
	return s
}

func (s *playfabStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.proxy == nil {
		n, _ := io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
		log.Printf("[astroneer] playfab stub %s %s body=%d (no tenant yet)", r.Method, r.URL.Path, n)
		http.Error(w, "playfab tenant not configured", http.StatusNotImplemented)
		return
	}
	log.Printf("[astroneer] playfab -> %s%s", s.target.Host, r.URL.Path)
	s.proxy.ServeHTTP(w, r)
}
