//go:build linux

package app

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func diskSpace(path string) (free, total uint64, err error) {
	var st syscall.Statfs_t
	err = syscall.Statfs(path, &st)
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize), err
}
func hostMemory() (total, used uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	var available uint64
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 2 {
			continue
		}
		n, _ := strconv.ParseUint(parts[1], 10, 64)
		if parts[0] == "MemTotal:" {
			total = n * 1024
		}
		if parts[0] == "MemAvailable:" {
			available = n * 1024
		}
	}
	if total >= available {
		used = total - available
	}
	return
}
func cpuCounters() (idle, total uint64) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	p := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
	for i, v := range p[1:] {
		if i >= 8 {
			break
		}
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 || i == 4 {
			idle += n
		}
	}
	return
}
