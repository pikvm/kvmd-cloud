package vars

import "fmt"

var (
	AppName = ""
	Version = "dev"
	Commit  = ""
	_build  = "debug"
	Debug   = true
)

func init() {
	if _build == "release" {
		Debug = false
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
