//go:build !windows && !linux

package app

import "errors"

func LockData(string) (func(), error) {
	return nil, errors.New("unsupported platform; use Linux or the Windows demo")
}
