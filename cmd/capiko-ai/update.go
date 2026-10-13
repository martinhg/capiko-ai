package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/release"
	"github.com/martinhg/capiko-ai/internal/tui"
)

// updateInputs bundles everything updateCommand needs so tests can stub it.
type updateInputs struct {
	capikoVersion  string
	engramVersion  string
	engramDetected bool
}

// fetchLatestVersions is a package-level var so tests can stub the network.
var fetchLatestVersions = func(ctx context.Context) (capikoLatest, engramLatest string) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	type result struct {
		capiko string
		engram string
	}
	ch := make(chan result, 1)
	go func() {
		var r result
		r.capiko, _ = release.Latest(ctx)
		r.engram, _ = engram.LatestVersion(ctx)
		ch <- r
	}()

	select {
	case r := <-ch:
		return r.capiko, r.engram
	case <-ctx.Done():
		return "", ""
	}
}

// updateCommand prints installed vs latest version for capiko-ai and engram.
//
//	capiko-ai update
func updateCommand(name string, _ []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "update" {
		return false, 0, nil
	}

	in := updateInputs{
		capikoVersion:  tui.Version,
		engramVersion:  engram.InstalledVersion(),
		engramDetected: engram.Detected(),
	}

	capikoLatest, engramLatest := fetchLatestVersions(context.Background())

	fmt.Fprintf(out, "capiko-ai  %s\n", versionLine(in.capikoVersion, capikoLatest))
	if in.engramDetected {
		fmt.Fprintf(out, "engram     %s\n", versionLine(in.engramVersion, engramLatest))
	} else {
		fmt.Fprintln(out, "engram     not installed")
	}

	return true, 0, nil
}

func versionLine(installed, latest string) string {
	if installed == "" {
		return "unknown"
	}
	if latest == "" {
		return installed
	}
	if release.IsNewer(installed, latest) {
		return fmt.Sprintf("%s (latest: %s)", installed, latest)
	}
	return fmt.Sprintf("%s (up to date)", installed)
}
