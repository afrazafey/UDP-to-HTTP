package buildinfo

import (
	"runtime/debug"
	"time"
)

var StartedAt = time.Now().UTC()

var Version = "0.4.0-dev"
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
		Commit = "unlinked-source"
	}
	if Date == "" {
		Date = "unlinked-source; use release build script"
	}
}
