package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testStub() *playfabStub {
	s := &playfabStub{now: func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }}
	s.key = make([]byte, 32)
	for i := range s.key {
		s.key[i] = byte(i + 1)
	}
	return s
}

func doPlayFab(t *testing.T, s *playfabStub, host, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, strings.NewReader(body))
	r.Host = host
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

type envelope struct {
	Code  int             `json:"code"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a PlayFab envelope: %v (%q)", err, w.Body.String())
	}
	return e
}

// Every fake TitleId subdomain and every TitleId body value must succeed
// identically: the service never gates on either.
func TestTitleIdAgnostic(t *testing.T) {
	s := testStub()
	hosts := []string{"FAKE1.playfabapi.com", "zz999.playfabapi.com", "ABCDE.playfabapi.com:443", "not-playfab.example.com", "127.0.0.1:24560"}
	bodies := []string{
		`{"TitleId":"FAKE1","NintendoSwitchDeviceId":"dev-1","CreateAccount":true}`,
		`{"TitleId":"something-else","NintendoSwitchDeviceId":"dev-1","CreateAccount":true}`,
		`{"NintendoSwitchDeviceId":"dev-1"}`,
	}
	var want string
	for _, host := range hosts {
		for _, body := range bodies {
			w := doPlayFab(t, s, host, "/Client/LoginWithNintendoSwitchDeviceId", body)
			if w.Code != 200 {
				t.Fatalf("host %s body %s: status %d, want 200 (%s)", host, body, w.Code, w.Body.String())
			}
			e := decodeEnvelope(t, w)
			if e.Code != 200 {
				t.Fatalf("host %s body %s: envelope code %d", host, body, e.Code)
			}
			var login struct {
				PlayFabID   string `json:"PlayFabId"`
				Ticket      string `json:"SessionTicket"`
				EntityToken struct {
					Token string `json:"EntityToken"`
				} `json:"EntityToken"`
			}
			if err := json.Unmarshal(e.Data, &login); err != nil {
				t.Fatalf("host %s: bad login data: %v", host, err)
			}
			if login.PlayFabID == "" || login.Ticket == "" || login.EntityToken.Token == "" {
				t.Fatalf("host %s: incomplete login result: %s", host, e.Data)
			}
			if want == "" {
				want = login.PlayFabID
			} else if login.PlayFabID != want {
				t.Fatalf("host %s body %s: PlayFabId %s, want stable %s", host, body, login.PlayFabID, want)
			}
		}
	}
}

func TestPlayFabPathsAlias(t *testing.T) {
	s := testStub()
	body := `{"TitleId":"X","NintendoSwitchDeviceId":"dev-9","CreateAccount":true}`
	direct := doPlayFab(t, s, "q.playfabapi.com", "/Client/LoginWithNintendoSwitchDeviceId", body)
	aliased := doPlayFab(t, s, "q.playfabapi.com", "/playfab/LoginWithNintendoSwitchDeviceId", body)
	aliasedFull := doPlayFab(t, s, "q.playfabapi.com", "/playfab/Client/LoginWithNintendoSwitchDeviceId", body)
	for name, w := range map[string]*httptest.ResponseRecorder{"direct": direct, "alias": aliased, "alias-full": aliasedFull} {
		if w.Code != 200 {
			t.Fatalf("%s: status %d, want 200", name, w.Code)
		}
	}
	if decodeEnvelope(t, direct).Data == nil || decodeEnvelope(t, aliased).Data == nil {
		t.Fatal("missing data")
	}
	var a, b struct {
		PlayFabID string `json:"PlayFabId"`
	}
	_ = json.Unmarshal(decodeEnvelope(t, direct).Data, &a)
	_ = json.Unmarshal(decodeEnvelope(t, aliased).Data, &b)
	if a.PlayFabID != b.PlayFabID {
		t.Fatalf("path alias changed identity: %s vs %s", a.PlayFabID, b.PlayFabID)
	}
}

func TestPlayFabLoginFlows(t *testing.T) {
	s := testStub()
	cases := []struct {
		path, body string
	}{
		{"/Client/LoginWithNintendoServiceAccount", `{"TitleId":"whatever","IdentityToken":"tok-1","CreateAccount":true}`},
		{"/Client/LoginWithNintendoSwitchDeviceId", `{"TitleId":"whatever","NintendoSwitchDeviceId":"dev-2","CreateAccount":true}`},
		{"/Client/LoginWithCustomID", `{"TitleId":"whatever","CustomId":"c-3","CreateAccount":true}`},
		{"/Client/GetPlayFabIDsFromNintendoSwitchDeviceIds", `{"TitleId":"whatever","NintendoSwitchDeviceIds":["d1","d2"]}`},
		{"/Client/GetPhotonAuthenticationToken", `{"TitleId":"whatever","PhotonApplicationId":"app"}`},
		{"/Client/GetTitleData", `{"TitleId":"whatever"}`},
		{"/Client/GetUserData", `{"TitleId":"whatever","PlayFabId":"X"}`},
		{"/Client/GetUserReadOnlyData", `{"TitleId":"whatever","PlayFabId":"X"}`},
		{"/Client/GetPlayerCombinedInfo", `{"TitleId":"whatever","PlayFabId":"X"}`},
		{"/Client/GetAccountInfo", `{"TitleId":"whatever","PlayFabId":"X"}`},
		{"/Authentication/GetEntityToken", `{}`},
		// Unknown client operations still succeed with empty data so the
		// game proceeds; the op is logged for follow-up implementation.
		{"/Client/GetFutureOperation", `{"TitleId":"whatever"}`},
	}
	for _, c := range cases {
		w := doPlayFab(t, s, "mystery.playfabapi.com", c.path, c.body)
		if w.Code != 200 {
			t.Errorf("%s: status %d, want 200 (%s)", c.path, w.Code, w.Body.String())
			continue
		}
		if e := decodeEnvelope(t, w); e.Code != 200 {
			t.Errorf("%s: envelope code %d", c.path, e.Code)
		}
	}
}

// A session ticket from login must round-trip into GetEntityToken and
// identify the same player.
func TestPlayFabTicketRoundTrip(t *testing.T) {
	s := testStub()
	w := doPlayFab(t, s, "t.playfabapi.com", "/Client/LoginWithNintendoSwitchDeviceId", `{"TitleId":"T","NintendoSwitchDeviceId":"roundtrip-1"}`)
	var login struct {
		PlayFabID string `json:"PlayFabId"`
		Ticket    string `json:"SessionTicket"`
		Entity    struct {
			Entity struct {
				ID string `json:"Id"`
			} `json:"Entity"`
		} `json:"EntityToken"`
	}
	_ = json.Unmarshal(decodeEnvelope(t, w).Data, &login)

	r := httptest.NewRequest(http.MethodPost, "https://t.playfabapi.com/Authentication/GetEntityToken", strings.NewReader(`{}`))
	r.Host = "other.playfabapi.com" // TitleId still must not matter
	r.Header.Set("X-Authentication", login.Ticket)
	w2 := httptest.NewRecorder()
	s.ServeHTTP(w2, r)
	if w2.Code != 200 {
		t.Fatalf("GetEntityToken: status %d (%s)", w2.Code, w2.Body.String())
	}
	var tok struct {
		Entity struct {
			ID string `json:"Id"`
		} `json:"Entity"`
	}
	_ = json.Unmarshal(decodeEnvelope(t, w2).Data, &tok)
	if tok.Entity.ID != login.Entity.Entity.ID {
		t.Fatalf("entity changed across round trip: %s vs %s", tok.Entity.ID, login.Entity.Entity.ID)
	}
}

func TestPlayFabRejectsGarbage(t *testing.T) {
	s := testStub()
	// Non-POST.
	r := httptest.NewRequest(http.MethodGet, "https://x.playfabapi.com/Client/GetTitleData", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: status %d, want 405", w.Code)
	}
	// Malformed path under the alias.
	w = doPlayFab(t, s, "x.playfabapi.com", "/playfab/", `{}`)
	if w.Code == 200 {
		t.Error("/playfab/ with no operation: got 200, want an error")
	}
	// Login without any credential cannot identify a player.
	w = doPlayFab(t, s, "x.playfabapi.com", "/Client/LoginWithNintendoSwitchDeviceId", `{"TitleId":"T"}`)
	if w.Code != 400 {
		t.Errorf("credential-less login: status %d, want 400", w.Code)
	}
	// Non-JSON body.
	w = doPlayFab(t, s, "x.playfabapi.com", "/Client/GetTitleData", `not json`)
	if w.Code != 400 {
		t.Errorf("non-JSON body: status %d, want 400", w.Code)
	}
}

func TestProxyModePreserved(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Client/LoginWithCustomID" {
			t.Errorf("proxied path %s, want /Client/LoginWithCustomID", r.URL.Path)
		}
		w.Write([]byte(`ok`))
	}))
	defer backend.Close()
	cfg := &Config{PlayFabURL: backend.URL}
	s := newPlayFabStub(cfg)
	if s.local() {
		t.Fatal("proxy configured but stub stayed local")
	}
	direct := testStub()
	if !direct.local() {
		t.Fatal("empty URL must serve locally")
	}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/playfab/LoginWithCustomID", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("proxy: status %d body %q", w.Code, w.Body.String())
	}
}
