//go:build linux

package checks

import (
	"bib/internal/capcheck"
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ResourcesChecker struct{}

func (r ResourcesChecker) ID() capcheck.CapabilityID { return "resources" }
func (r ResourcesChecker) Description() string {
	return "Reports available CPU and memory, respecting cgroup limits when present (Linux)"
}

func (r ResourcesChecker) Check(ctx context.Context) capcheck.CheckResult {
	start := time.Now()
	res := capcheck.CheckResult{
		ID:      r.ID(),
		Name:    "Resources",
		Details: map[string]any{},
	}

	cpuLimit, cpuMethod := detectCPULimit()
	memLimit, memMethod := detectMemLimit()

	res.Details["cpu_cores_effective"] = cpuLimit
	res.Details["cpu_detection_method"] = cpuMethod

	res.Details["memory_bytes_effective"] = memLimit
	res.Details["memory_detection_method"] = memMethod

	if cpuLimit > 0 && memLimit > 0 {
		res.Supported = true
	} else {
		res.Supported = false
		res.Error = "failed to determine cpu or memory limits"
	}

	res.Duration = time.Since(start)
	return res
}

// CPU detection

func detectCPULimit() (float64, string) {
	if isCgroupV2() {
		if cores, ok := readCPUv2(); ok {
			return cores, "cgroupv2"
		}
	}
	if cores, ok := readCPUv1(); ok {
		return cores, "cgroupv1"
	}
	return float64(runtime.NumCPU()), "host_numcpu"
}

func readCPUv2() (float64, bool) {
	cpuMax := "/sys/fs/cgroup/cpu.max"
	data, err := os.ReadFile(cpuMax)
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(bytes.TrimSpace(data)))
	if len(fields) != 2 {
		return 0, false
	}
	if fields[0] == "max" {
		return float64(runtime.NumCPU()), true
	}
	quota, err1 := strconv.ParseFloat(fields[0], 64)
	period, err2 := strconv.ParseFloat(fields[1], 64)
	if err1 != nil || err2 != nil || period == 0 {
		return 0, false
	}
	cores := quota / period
	if cores <= 0 {
		return 0, false
	}
	// Respect cpuset if available (min of quota-derived cores and cpuset count)
	if cs, ok := readCPUSetCountV2(); ok && cs > 0 && float64(cs) < cores {
		cores = float64(cs)
	}
	return cores, true
}

func readCPUSetCountV2() (int, bool) {
	path := "/sys/fs/cgroup/cpuset.cpus.effective"
	return parseCPUSetList(path)
}

func readCPUv1() (float64, bool) {
	base := "/sys/fs/cgroup"
	quotaFile := filepath.Join(base, "cpu", "cpu.cfs_quota_us")
	periodFile := filepath.Join(base, "cpu", "cpu.cfs_period_us")

	quota, ok1 := readInt(quotaFile)
	period, ok2 := readInt(periodFile)
	if !ok1 || !ok2 || period <= 0 {
		return 0, false
	}
	if quota == -1 {
		cores := float64(runtime.NumCPU())
		if cs, ok := readCPUSetCountV1(); ok && cs > 0 && float64(cs) < cores {
			cores = float64(cs)
		}
		return cores, true
	}
	cores := float64(quota) / float64(period)
	if cores <= 0 {
		return 0, false
	}
	if cs, ok := readCPUSetCountV1(); ok && cs > 0 && float64(cs) < cores {
		cores = float64(cs)
	}
	return cores, true
}

func readCPUSetCountV1() (int, bool) {
	path := "/sys/fs/cgroup/cpuset/cpuset.cpus"
	return parseCPUSetList(path)
}

func parseCPUSetList(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	// Format examples: "0-3", "0,2,4-6"
	val := strings.TrimSpace(string(data))
	count := 0
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			ends := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(ends[0])
			end, err2 := strconv.Atoi(ends[1])
			if err1 != nil || err2 != nil || end < start {
				continue
			}
			count += end - start + 1
		} else {
			count++
		}
	}
	return count, true
}

// Memory detection

func detectMemLimit() (uint64, string) {
	if isCgroupV2() {
		if v, ok := readMemV2(); ok {
			return v, "cgroupv2"
		}
	}
	if v, ok := readMemV1(); ok {
		return v, "cgroupv1"
	}
	if v, ok := readProcMeminfo(); ok {
		return v, "/proc/meminfo"
	}
	return 0, "unknown"
}

func readMemV2() (uint64, bool) {
	file := "/sys/fs/cgroup/memory.max"
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, false
	}
	val := strings.TrimSpace(string(data))
	if val == "max" {
		return readProcMeminfo()
	}
	return parseUint(val)
}

func readMemV1() (uint64, bool) {
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	return readIntToUint(file)
}

func readProcMeminfo() (uint64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, false
				}
				return kb * 1024, true
			}
		}
	}
	return 0, false
}

func isCgroupV2() bool {
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}

func readInt(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	i, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}

func readIntToUint(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	return parseUint(strings.TrimSpace(string(data)))
}

func parseUint(s string) (uint64, bool) {
	u, err := strconv.ParseUint(s, 10, 64)
	return u, err == nil
}
