package vars

import "fmt"

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

	VersionString = fmt.Sprintf("%s %s [%s]", AppName, Version, Commit)
}
