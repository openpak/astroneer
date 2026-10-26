package main

import (
	"flag"
	"os"
	"strings"
)

// loadFlags registers every knob (flag first, environment as the default) and
// returns a Config once the flags are parsed. Called once from main.
func loadFlags() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.Addr, "addr", envOr("ASTRONEER_HTTP_ADDR", ":23560"),
		"plain HTTP listen address, behind the TLS terminator (env ASTRONEER_HTTP_ADDR)")
	flag.StringVar(&cfg.HealthAddr, "health-addr", envOr("ASTRONEER_HEALTH_ADDR", ":23561"),
		"health listen address (env ASTRONEER_HEALTH_ADDR)")
	flag.StringVar(&cfg.NexBase, "nex-base", envOr("ASTRONEER_NEX_BASE", "22200"),
		"switch-nex NEX port base for this title (env ASTRONEER_NEX_BASE)")
	flag.StringVar(&cfg.NexHost, "nex-host", envOr("ASTRONEER_NEX_HOST", "127.0.0.1"),
		"switch-nex host the console dials (env ASTRONEER_NEX_HOST)")
	flag.StringVar(&cfg.PlayFabURL, "playfab-url", envOr("ASTRONEER_PLAYFAB_URL", ""),
		"playfab shim base URL, empty until the TitleId tenant exists (env ASTRONEER_PLAYFAB_URL)")
	flag.StringVar(&cfg.TitleID, "title-id", envOr("ASTRONEER_TITLE_ID", "0100E63013E60000"),
		"Switch title ID this service answers for (env ASTRONEER_TITLE_ID)")
	flag.Parse()
	cfg.PlayFabURL = strings.TrimRight(cfg.PlayFabURL, "/")
	return cfg
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}
