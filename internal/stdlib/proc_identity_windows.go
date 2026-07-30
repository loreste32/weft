//go:build windows && !js

package stdlib

import "os/user"

type processIdentity struct {
	uid, gid     int64
	uidOK, gidOK bool
	user, group  string
}

func currentProcessIdentity() processIdentity {
	identity := processIdentity{}
	if current, err := user.Current(); err == nil {
		identity.user = current.Username
	}
	return identity
}
