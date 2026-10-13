package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/release"
)

// upgradeCommand upgrades the engram binary to the latest version via go install.
//
//	capiko-ai upgrade
func upgradeCommand(name string, _ []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "upgrade" {
		return false, 0, nil
	}

	if !engram.GoAvailable() {
		fmt.Fprintln(out, "error: go toolchain not found on PATH")
		fmt.Fprintln(out, "Install Go from https://go.dev/dl/ and retry.")
		return true, 1, nil
	}

	installed := engram.InstalledVersion()
	if installed != "" {
		fmt.Fprintf(out, "engram %s installed\n", installed)
	} else {
		fmt.Fprintln(out, "engram not installed — will install")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	latest, err := engram.LatestVersion(ctx)
	if err != nil {
		fmt.Fprintf(out, "warning: could not fetch latest version: %v\n", err)
		fmt.Fprintln(out, "upgrading to @latest…")
		latest = ""
	} else if installed != "" && !release.IsNewer(installed, latest) {
		fmt.Fprintf(out, "already up to date (%s)\n", installed)
		return true, 0, nil
	} else {
		fmt.Fprintf(out, "upgrading to %s…\n", latest)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()
	if err := engram.Upgrade(ctx2, latest); err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
		return true, 1, nil
	}

	after := engram.InstalledVersion()
	if after != "" {
		fmt.Fprintf(out, "engram %s ✓\n", after)
	} else {
		fmt.Fprintln(out, "installed ✓")
	}
	return true, 0, nil
}
