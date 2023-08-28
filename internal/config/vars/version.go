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

	if Commit == "" {
		VersionString = fmt.Sprintf("%s (ver: %s)", AppName, Version)
	} else {
		VersionString = fmt.Sprintf("%s (ver: %s [%s])", AppName, Version, Commit)
	}
}

func GetVersion() string {
	if Debug {
		return fmt.Sprintf("%s version %s @ %s", AppName, Version, Commit)
	} else {
		return fmt.Sprintf("%s version %s", AppName, Version)
	}
}

func PrintVersion() {
	println(GetVersion())
}
