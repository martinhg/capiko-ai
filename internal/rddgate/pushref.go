package rddgate

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// deletedSHA is the all-zero SHA git uses to mark a deleted ref in the
// pre-push protocol's local-sha field.
const deletedSHA = "0000000000000000000000000000000000000000"

// PushRef is one line from git's pre-push hook stdin protocol:
// "<local-ref> <local-sha> <remote-ref> <remote-sha>".
type PushRef struct {
	LocalRef  string
	LocalSHA  string
	RemoteRef string
	RemoteSHA string
}

// ParsePushRefs parses git's pre-push stdin protocol, one ref per line,
// fields separated by single spaces:
// <local-ref> SP <local-sha> SP <remote-ref> SP <remote-sha> LF
//
// Empty input (no refs to push) returns an empty, non-nil slice and a nil
// error. A malformed line (not exactly 4 whitespace-separated fields)
// returns an error immediately (fail-closed) — no partial results. It is a
// pure function: no I/O beyond reading from the supplied io.Reader.
func ParsePushRefs(r io.Reader) ([]PushRef, error) {
	refs := make([]PushRef, 0)

	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 4 {
			return nil, fmt.Errorf("rddgate: malformed pre-push stdin line %d: expected 4 fields, got %d: %q", lineNum, len(fields), line)
		}

		refs = append(refs, PushRef{
			LocalRef:  fields[0],
			LocalSHA:  fields[1],
			RemoteRef: fields[2],
			RemoteSHA: fields[3],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("rddgate: failed to read pre-push stdin: %w", err)
	}

	return refs, nil
}

// IsDeleteRef reports whether p represents a delete-ref push (the local
// SHA is all zeros, meaning nothing is being pushed — the remote ref is
// being deleted). Delete-refs push no new content, so gates skip them.
func IsDeleteRef(p PushRef) bool {
	return p.LocalSHA == deletedSHA
}
