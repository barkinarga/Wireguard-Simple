//go:build !linux && !windows

package app

import "errors"

func diskSpace(path string) (uint64, uint64, error) {
	return 0, 0, errors.New("disk statistics unavailable")
}
func hostMemory() (uint64, uint64)  { return 0, 0 }
func cpuCounters() (uint64, uint64) { return 0, 0 }
