package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PlayFab UE4 Client API for ASTRONEER (<TitleId>.playfabapi.com), served
// TitleId-agnostically: the real TitleId lives in cooked config inside the
// AES-encrypted pak and is still unknown, while prod DNS already routes
// *.playfabapi.com to us. The handler below therefore never parses, matches,
// or gates on a TitleId — neither the Host subdomain nor the TitleId field in
// the JSON body affects routing or the response. Both are only logged (the
// capture that will eventually identify the real TitleId) and otherwise
// ignored.
//
// Identity is stateless and deterministic: a PlayFabId/EntityId is derived
// from the credential the client presented (Nintendo service-account token,
// Switch device ID, or custom ID), and session/entity tokens are
// HMAC-signed with a per-boot key. Restarting the daemon keeps player IDs
// stable but invalidates outstanding tokens, which the client re-logins for.
//
// When ASTRONEER_PLAYFAB_URL points at the shared shim (only useful after the
// TitleId is recovered and the shim carries this title's tenant), requests
// are reverse-proxied there instead of being answered locally.

const playfabMaxBody = 64 << 10

type playfabStub struct {
	url    string // proxy target; empty = answer locally (TitleId-agnostic)
	proxy  *httputil.ReverseProxy
	target *url.URL
	key    []byte // per-boot HMAC key for local tokens
	now    func() time.Time
}

func newPlayFabStub(cfg *Config) *playfabStub {
	s := &playfabStub{now: time.Now}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Printf("[astroneer] !! no entropy for playfab tokens: %v", err)
	}
	s.key = key
	if cfg.PlayFabURL == "" {
		return s
	}
	u, err := url.Parse(cfg.PlayFabURL)
	if err != nil {
		log.Printf("[astroneer] !! bad playfab url %q: %v (answering locally)", cfg.PlayFabURL, err)
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

func (s *playfabStub) local() bool { return s.proxy == nil }

func (s *playfabStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.local() {
		log.Printf("[astroneer] playfab -> %s%s", s.target.Host, r.URL.Path)
		s.proxy.ServeHTTP(w, r)
		return
	}
	s.serveLocal(w, r)
}

// serveLocal answers the PlayFab API without any TitleId check. It accepts
// the game's native paths (/Client/*, /Authentication/*) as well as the
// /playfab/*-prefixed form this daemon historically exposed; the latter
// carries the operation with or without the /Client segment.
func (s *playfabStub) serveLocal(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if rest, ok := strings.CutPrefix(path, "/playfab"); ok {
		path = rest
		if !strings.HasPrefix(path, "/Client/") && !strings.HasPrefix(path, "/Authentication/") {
			path = "/Client" + rest
		}
	}
	rec := &playfabStatus{ResponseWriter: w, code: 200}
	defer func() {
		// Log only the fixed operation and the presented TitleIds (Host
		// subdomain + body field): the capture for TitleId recovery. Never
		// bodies, tokens, device IDs, or queries.
		log.Printf("[astroneer] playfab host=%s operation=%s -> %d", playfabHostTag(r), r.URL.Path, rec.code)
	}()
	if r.Method != http.MethodPost {
		rec.Header().Set("Allow", "POST")
		playfabFailure(rec, 405, "InvalidParams", 1000, "Use POST.")
		return
	}
	if _, _, ok := splitPlayFab(path); !ok {
		playfabFailure(rec, 400, "InvalidParams", 1000, "Expected /Client/<Operation> or /Authentication/<Operation>.")
		return
	}
	switch path {
	case "/Client/LoginWithNintendoServiceAccount":
		s.login(rec, r, "service-account")
	case "/Client/LoginWithNintendoSwitchDeviceId":
		s.login(rec, r, "switch-device")
	case "/Client/LoginWithCustomID":
		s.login(rec, r, "custom")
	case "/Client/GetPlayFabIDsFromNintendoSwitchDeviceIds":
		s.getIDsFromSwitchDevices(rec, r)
	case "/Client/GetPhotonAuthenticationToken":
		s.photonToken(rec, r)
	case "/Client/GetTitleData":
		s.staticData(rec, r, map[string]any{"Data": map[string]string{}})
	case "/Client/GetUserData", "/Client/GetUserReadOnlyData":
		s.staticData(rec, r, map[string]any{"Data": map[string]any{}, "DataVersion": 0})
	case "/Client/GetPlayerCombinedInfo":
		s.combinedInfo(rec, r)
	case "/Client/GetAccountInfo":
		s.accountInfo(rec, r)
	case "/Authentication/GetEntityToken":
		s.entityToken(rec, r)
	default:
		// Unknown operation, well-formed envelope: answer success with empty
		// data so the client proceeds independent of TitleId, and log the
		// operation so the capture shows what to implement next.
		var in map[string]any
		if !playfabDecode(rec, r, &in) {
			return
		}
		log.Printf("[astroneer] playfab unimplemented %s (empty success)", path)
		playfabSuccess(rec, map[string]any{})
	}
}

func splitPlayFab(path string) (family, operation string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "" {
		return "", "", false
	}
	switch parts[1] {
	case "Client", "Authentication":
	default:
		return "", "", false
	}
	if parts[2] == "" || len(parts[2]) > 64 {
		return "", "", false
	}
	for _, c := range parts[2] {
		if c < 'A' || c > 'Z' {
			if c < 'a' || c > 'z' {
				return "", "", false
			}
		}
	}
	return parts[1], parts[2], true
}

// playfabHostTag is the Host subdomain presented by the client (the TitleId
// position), lowercased for the log; "" when the Host carries none.
func playfabHostTag(r *http.Request) string {
	host, _, _ := strings.Cut(r.Host, ":")
	host = strings.ToLower(host)
	sub, ok := strings.CutSuffix(host, ".playfabapi.com")
	if !ok || sub == "" || strings.Contains(sub, ".") {
		return "-"
	}
	return sub
}

type playfabStatus struct {
	http.ResponseWriter
	code int
}

func (w *playfabStatus) WriteHeader(code int) { w.code = code; w.ResponseWriter.WriteHeader(code) }

func playfabWrite(w http.ResponseWriter, code int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		code = http.StatusInternalServerError
		body = []byte(`{"code":500,"status":"InternalServerError","error":"InternalServerError","errorCode":1110,"errorMessage":"Response encoding failed."}`)
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func playfabSuccess(w http.ResponseWriter, data any) {
	playfabWrite(w, 200, map[string]any{"code": 200, "status": "OK", "data": data})
}

func playfabFailure(w http.ResponseWriter, status int, name string, code int, message string) {
	playfabWrite(w, status, map[string]any{"code": status, "status": http.StatusText(status), "error": name, "errorCode": code, "errorMessage": message})
}

func playfabDecode(w http.ResponseWriter, r *http.Request, out any) bool {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, playfabMaxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			playfabFailure(w, 413, "InvalidParams", 1000, "Request body too large.")
		} else {
			playfabFailure(w, 400, "InvalidParams", 1000, "Invalid request body.")
		}
		return false
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' || json.Unmarshal(b, out) != nil {
		playfabFailure(w, 400, "InvalidParams", 1000, "Expected one JSON object.")
		return false
	}
	return true
}

type playfabEntityKey struct {
	ID   string `json:"Id"`
	Type string `json:"Type"`
}

type playfabEntityResult struct {
	Entity          playfabEntityKey `json:"Entity"`
	EntityToken     string           `json:"EntityToken"`
	TokenExpiration time.Time        `json:"TokenExpiration"`
}

type playfabLoginResult struct {
	PlayFabID       string              `json:"PlayFabId"`
	SessionTicket   string              `json:"SessionTicket"`
	EntityToken     playfabEntityResult `json:"EntityToken"`
	NewlyCreated    bool                `json:"NewlyCreated"`
	LastLoginTime   time.Time           `json:"LastLoginTime"`
	SettingsForUser map[string]bool     `json:"SettingsForUser"`
}

// playfabID derives a stable player ID from the presented credential. The
// TitleId is deliberately not an input: the same console keeps its ID
// whatever TitleId the client was configured with.
func playfabID(kind, credential string) (playfabID, entityID string) {
	sum := sha256.Sum256([]byte("openpak/astroneer|" + kind + "|" + credential))
	hexsum := strings.ToUpper(hex.EncodeToString(sum[:]))
	return hexsum[:16], "E" + hexsum[16:48]
}

func (s *playfabStub) login(w http.ResponseWriter, r *http.Request, kind string) {
	var in struct {
		// TitleId is read and ignored: any value (or none) is accepted.
		TitleID                string `json:"TitleId"`
		IdentityToken          string `json:"IdentityToken"`
		NintendoSwitchDeviceId string `json:"NintendoSwitchDeviceId"`
		CustomID               string `json:"CustomId"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	var credential string
	switch kind {
	case "service-account":
		credential = in.IdentityToken
	case "switch-device":
		credential = in.NintendoSwitchDeviceId
	case "custom":
		credential = in.CustomID
	}
	if credential == "" || len(credential) > 8192 {
		playfabFailure(w, 400, "InvalidParams", 1000, "A login credential is required.")
		return
	}
	now := s.now().UTC().Truncate(time.Second)
	pid, eid := playfabID(kind, credential)
	session, _, _ := s.issue(pid, eid, "session", now)
	entity, expiry, _ := s.issue(pid, eid, "entity", now)
	playfabSuccess(w, playfabLoginResult{
		PlayFabID:     pid,
		SessionTicket: session,
		EntityToken: playfabEntityResult{
			Entity:          playfabEntityKey{ID: eid, Type: "title_player_account"},
			EntityToken:     entity,
			TokenExpiration: expiry,
		},
		NewlyCreated:  false,
		LastLoginTime: now,
		SettingsForUser: map[string]bool{
			"GatherDeviceInfo": false,
			"GatherFocusInfo":  false,
			"NeedsAttribution": false,
		},
	})
}

// staticData answers fixed-shape reads (title/user data). The body is still
// validated as one JSON object so malformed calls fail like the real API.
func (s *playfabStub) staticData(w http.ResponseWriter, r *http.Request, data map[string]any) {
	var in map[string]any
	if !playfabDecode(w, r, &in) {
		return
	}
	playfabSuccess(w, data)
}

func (s *playfabStub) getIDsFromSwitchDevices(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TitleID                 string   `json:"TitleId"`
		NintendoSwitchDeviceIds []string `json:"NintendoSwitchDeviceIds"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	type mapping struct {
		NintendoSwitchDeviceId string `json:"NintendoSwitchDeviceId"`
		PlayFabID              string `json:"PlayFabId"`
	}
	out := make([]mapping, 0, len(in.NintendoSwitchDeviceIds))
	for _, id := range in.NintendoSwitchDeviceIds {
		if id == "" || len(id) > 256 || len(out) >= 100 {
			continue
		}
		pid, _ := playfabID("switch-device", id)
		out = append(out, mapping{id, pid})
	}
	playfabSuccess(w, map[string]any{"Data": out})
}

func (s *playfabStub) photonToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TitleID             string `json:"TitleId"`
		PhotonApplicationId string `json:"PhotonApplicationId"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	// No Photon backend is linked (see docs/protocol.md); the token only has
	// to be well-formed and stable per (app, console) so the client proceeds.
	caller := r.Header.Get("X-Authentication")
	if caller == "" {
		caller = "anonymous"
	}
	sum := sha256.Sum256([]byte("openpak/astroneer|photon|" + in.PhotonApplicationId + "|" + caller))
	playfabSuccess(w, map[string]any{
		"PhotonAuthenticationToken": "astro-" + base64.RawURLEncoding.EncodeToString(sum[:24]),
	})
}

func (s *playfabStub) combinedInfo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TitleID   string `json:"TitleId"`
		PlayFabID string `json:"PlayFabId"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	pid := in.PlayFabID
	if c, ok := s.subjectFromTicket(r); ok {
		pid = c
	}
	if pid == "" {
		pid, _ = playfabID("custom", "anonymous")
	}
	playfabSuccess(w, map[string]any{
		"PlayFabId":         pid,
		"InfoResultPayload": map[string]any{},
		"SettingsForUser":   map[string]bool{"NeedsAttribution": false},
	})
}

func (s *playfabStub) accountInfo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TitleID   string `json:"TitleId"`
		PlayFabID string `json:"PlayFabId"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	pid := in.PlayFabID
	if c, ok := s.subjectFromTicket(r); ok {
		pid = c
	}
	if pid == "" {
		pid, _ = playfabID("custom", "anonymous")
	}
	playfabSuccess(w, map[string]any{
		"AccountInfo": map[string]any{"PlayFabId": pid},
	})
}

func (s *playfabStub) entityToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Entity *playfabEntityKey `json:"Entity"`
	}
	if !playfabDecode(w, r, &in) {
		return
	}
	now := s.now().UTC().Truncate(time.Second)
	pid, eid := playfabID("custom", "anonymous")
	if tickets := r.Header.Values("X-Authentication"); len(tickets) == 1 {
		if p, e, ok := s.verify(tickets[0], "session", now); ok {
			pid, eid = p, e
		}
	} else if entities := r.Header.Values("X-EntityToken"); len(entities) == 1 {
		if p, e, ok := s.verify(entities[0], "entity", now); ok {
			pid, eid = p, e
		}
	}
	if in.Entity != nil && in.Entity.ID != "" && in.Entity.ID != eid {
		// A foreign entity was requested; stay on our own subject rather
		// than minting for an ID we did not derive.
		playfabFailure(w, 403, "NotAuthenticated", 1074, "Requested entity does not match this credential.")
		return
	}
	token, expiry, _ := s.issue(pid, eid, "entity", now)
	playfabSuccess(w, playfabEntityResult{
		Entity:          playfabEntityKey{ID: eid, Type: "title_player_account"},
		EntityToken:     token,
		TokenExpiration: expiry,
	})
}

// subjectFromTicket returns the PlayFabId of a verifiable local session
// ticket presented on the request, if any.
func (s *playfabStub) subjectFromTicket(r *http.Request) (string, bool) {
	tickets := r.Header.Values("X-Authentication")
	if len(tickets) != 1 {
		return "", false
	}
	p, _, ok := s.verify(tickets[0], "session", s.now().UTC())
	if !ok {
		return "", false
	}
	return p, true
}

const playfabTokenLifetime = time.Hour

type playfabCredential struct {
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	Entity   string `json:"entity"`
	Issued   int64  `json:"iat"`
	Expiry   int64  `json:"exp"`
}

func playfabTokenPrefix(audience string) string {
	if audience == "session" {
		return "oast1"
	}
	return "oaet1"
}

// issue mints a stateless HMAC-signed token. These are local opaque
// credentials, not Nintendo or Microsoft tokens.
func (s *playfabStub) issue(subject, entity, audience string, now time.Time) (string, time.Time, error) {
	expiry := now.Add(playfabTokenLifetime)
	c := playfabCredential{audience, subject, entity, now.Unix(), expiry.Unix()}
	b, err := json.Marshal(c)
	if err != nil {
		return "", time.Time{}, err
	}
	input := playfabTokenPrefix(audience) + "." + base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expiry, nil
}

func (s *playfabStub) verify(token, audience string, now time.Time) (subject, entity string, ok bool) {
	if len(token) > 8192 {
		return "", "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != playfabTokenPrefix(audience) {
		return "", "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	var c playfabCredential
	if json.Unmarshal(raw, &c) != nil || c.Audience != audience {
		return "", "", false
	}
	if now.Unix() > c.Expiry || c.Subject == "" || c.Entity == "" {
		return "", "", false
	}
	return c.Subject, c.Entity, true
}
