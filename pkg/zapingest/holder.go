package zapingest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// holderOf names the process already listening on addr, so a bind collision
// says WHO it lost to instead of just "address already in use".
//
// This exists because the failure it describes is invisible otherwise. Two
// implementations of one ingest edge, linked into one binary, race for
// :4317-:4319; whichever binds first wins and the loser's telemetry silently
// goes nowhere. "address already in use" does not say that the winner is the
// same PID — and when it is, the answer is not "retry" or "pick another port",
// it is "delete one of the two implementations".
//
// Returns "" when the holder cannot be determined (no procfs, or the socket
// belongs to another user). The caller must still report the bind error; this
// only enriches it.
func holderOf(addr string) string {
	_, portStr, err := splitPort(addr)
	if err != nil {
		return ""
	}
	inodes := listenInodes(portStr)
	if len(inodes) == 0 {
		return ""
	}
	pid, ok := pidForInodes(inodes)
	if !ok {
		return ""
	}

	desc := fmt.Sprintf("pid %d", pid)
	if cmd := processName(pid); cmd != "" {
		desc += " (" + cmd + ")"
	}
	if pid == os.Getpid() {
		desc += " — THIS SAME PROCESS: two ingest implementations are linked into one binary, and only one may own the edge"
	}
	return desc
}

func splitPort(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return "", "", fmt.Errorf("no port in %q", addr)
	}
	p := addr[i+1:]
	if _, err := strconv.Atoi(p); err != nil {
		return "", "", err
	}
	return addr[:i], p, nil
}

// listenInodes returns the socket inodes LISTENing on port, across IPv4 and
// IPv6. procfs writes the port as uppercase hex.
func listenInodes(port string) map[string]struct{} {
	n, err := strconv.Atoi(port)
	if err != nil {
		return nil
	}
	want := fmt.Sprintf("%04X", n)
	out := map[string]struct{}{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Scan() // header
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 {
				continue
			}
			// fields[1] = local hexaddr:hexport, fields[3] = state (0A = LISTEN),
			// fields[9] = inode.
			local := fields[1]
			if i := strings.LastIndex(local, ":"); i < 0 || local[i+1:] != want {
				continue
			}
			if fields[3] != "0A" {
				continue
			}
			out[fields[9]] = struct{}{}
		}
		fh.Close()
	}
	return out
}

// pidForInodes finds the process holding any of these socket inodes. Our own
// PID is preferred when it matches, because "the holder is us" is the single
// most useful thing this function can report.
func pidForInodes(inodes map[string]struct{}) (int, bool) {
	self := os.Getpid()
	if holdsAny(self, inodes) {
		return self, true
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if holdsAny(pid, inodes) {
			return pid, true
		}
	}
	return 0, false
}

func holdsAny(pid int, inodes map[string]struct{}) bool {
	fds, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "fd"))
	if err != nil {
		return false
	}
	for _, fd := range fds {
		link, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", fd.Name()))
		if err != nil {
			continue
		}
		ino, ok := strings.CutPrefix(link, "socket:[")
		if !ok {
			continue
		}
		if _, hit := inodes[strings.TrimSuffix(ino, "]")]; hit {
			return true
		}
	}
	return false
}

func processName(pid int) string {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err == nil && len(b) > 0 {
		return strings.Join(strings.FieldsFunc(string(b), func(r rune) bool { return r == 0 }), " ")
	}
	b, err = os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
