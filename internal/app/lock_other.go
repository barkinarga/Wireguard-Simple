//go:build !windows && !linux

package app

import "errors"

func LockData(string) (func(), error) {
	return nil, errors.New("bu platform desteklenmiyor; Linux veya Windows demo kullanın")
}
