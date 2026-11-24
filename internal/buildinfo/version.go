package buildinfo

import (
	"runtime/debug"
	"strings"
	"time"
)

var StartedAt = time.Now().UTC()

const (
	Name      = "UDPHTTP Proxy"
	Developer = "Afraaz"
	Website   = "https://afraaz.xyz"
)

var Version = "0.4.2-rc.1"
var Commit = ""
var Date = ""

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		dirty := false
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && Commit == "" {
				Commit = setting.Value
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				dirty = true
			}
		}
		if dirty && Commit != "" && len(Commit) == 40 {
			Commit += "+dirty"
		}
	}
	if Commit == "" {
		Commit = "unknown"
	}
}

func Environment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development":
		return "Development"
	case "staging":
		return "Staging"
	default:
		return "Production"
	}
}

func Maturity() string {
	switch {
	case strings.Contains(Version, "-rc"):
		return "Release candidate"
	case strings.Contains(Version, "-"):
		return "Development"
	default:
		return "Versioned build"
	}
}
