//go:build !windows && !js

package stdlib

import (
	"os"
	"os/user"
	"strconv"
)

type processIdentity struct {
	uid, gid     int64
	uidOK, gidOK bool
	user, group  string
}

func currentProcessIdentity() processIdentity {
	identity := processIdentity{
		uid:   int64(os.Getuid()),
		gid:   int64(os.Getgid()),
		uidOK: true,
		gidOK: true,
	}
	if current, err := user.LookupId(strconv.FormatInt(identity.uid, 10)); err == nil {
		identity.user = current.Username
	}
	if current, err := user.LookupGroupId(strconv.FormatInt(identity.gid, 10)); err == nil {
		identity.group = current.Name
	}
	return identity
}
