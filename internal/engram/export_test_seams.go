package engram

import "context"

// ExportRunGoInstall returns the current runGoInstall function for test backup.
func ExportRunGoInstall() func(context.Context, string) (string, error) {
	return runGoInstall
}

// SetRunGoInstall replaces the runGoInstall function for testing.
func SetRunGoInstall(fn func(context.Context, string) (string, error)) {
	runGoInstall = fn
}

// ExportLatestURL returns the current latestURL for test backup.
func ExportLatestURL() string { return latestURL }

// SetLatestURL replaces latestURL for testing.
func SetLatestURL(u string) { latestURL = u }

// ExportRunOut returns the current runOut function for test backup.
func ExportRunOut() func(...string) (string, error) { return runOut }

// SetRunOut replaces runOut for testing.
func SetRunOut(fn func(...string) (string, error)) { runOut = fn }
