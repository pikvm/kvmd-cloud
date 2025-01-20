package vars

import "github.com/segmentio/ksuid"

var (
	InstanceUUID string
)

func init() {
	InstanceUUID = ksuid.New().String()
}
