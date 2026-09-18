//go:build !windows

package models

func isDownloadSharingError(error) bool { return false }
