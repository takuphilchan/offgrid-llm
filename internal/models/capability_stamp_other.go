//go:build !linux

package models

import "os"

type capabilityFileStamp struct{}

func capabilityChangeStamp(*os.File) (capabilityFileStamp, bool) {
	return capabilityFileStamp{}, false
}
