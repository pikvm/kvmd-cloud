package vars

import (
	"github.com/google/uuid"
)

var (
	InstanceUUID string
)

func init() {
	uuid := uuid.New()
	InstanceUUID = uuid.String()
}
