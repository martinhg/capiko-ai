package main

import "github.com/martinhg/capiko-ai/internal/copilot"

// requireHost detects the Copilot CLI and maps the result to an exit code,
// so every headless command (install/sync/uninstall) gates on the same
// contract instead of each reimplementing it. detect is injected so this is
// testable without touching the real PATH or home directory.
//
// Returns (host, 0) on success, (nil, 2) when detect reports Copilot is not
// installed/initialized ((nil, nil)), and (nil, 1) when detect itself errors.
func requireHost(detect func() (*copilot.Host, error)) (host *copilot.Host, exitCode int) {
	h, err := detect()
	if err != nil {
		return nil, 1
	}
	if h == nil {
		return nil, 2
	}
	return h, 0
}
