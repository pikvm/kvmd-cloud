package vars

import (
	"fmt"
	"strings"
)

var (
	AppName       = ""
	Version       = "dev"
	Commit        = ""
	CommitSHA     = ""
	_build        = "debug"
	Debug         = true
	VersionString = ""
)

func init() {
	if _build == "release" {
		Debug = false
	}

	Version, _ = strings.CutPrefix(Version, "v")
	if Commit == "" {
		VersionString = fmt.Sprintf("%s version %s", AppName, Version)
	} else {
		VersionString = fmt.Sprintf("%s version %s [%s]", AppName, Version, Commit)
	}
}
