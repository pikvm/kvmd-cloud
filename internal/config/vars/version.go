package vars

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	AppName         = ""
	Version         = "dev"
	Commit          = ""
	CommitSHA       = ""
	_buildTimestamp = ""
	BuildTime       time.Time
	_build          = "debug"
	Debug           = true
	VersionString   = ""
)

func init() {
	if _build == "release" {
		Debug = false
	}

	if ts, err := strconv.ParseInt(_buildTimestamp, 10, 64); err != nil {
		BuildTime = time.Unix(ts, 0)
	}

	Version, _ = strings.CutPrefix(Version, "v")
	if Commit == "" {
		VersionString = fmt.Sprintf("%s version %s [%s]", AppName, Version, _build)
	} else {
		VersionString = fmt.Sprintf("%s version %s [%s] [%s]", AppName, Version, Commit, _build)
	}
}
