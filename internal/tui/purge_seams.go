package tui

// Exported accessors for purge seams — used by cmd/capiko-ai tests to stub
// the environment without touching the real filesystem.

func ExportPurgeGitDir() func(string) (string, error)       { return purgeGitDir }
func ExportPurgeGitCommonDir() func(string) (string, error) { return purgeGitCommonDir }
func ExportPurgeVSCodeUserPath() func() (string, error)     { return purgeVSCodeUserPath }
func ExportPurgeHomeDir() func() (string, error)            { return purgeHomeDir }

func SetPurgeGitDir(f func(string) (string, error))       { purgeGitDir = f }
func SetPurgeGitCommonDir(f func(string) (string, error)) { purgeGitCommonDir = f }
func SetPurgeVSCodeUserPath(f func() (string, error))     { purgeVSCodeUserPath = f }
func SetPurgeHomeDir(f func() (string, error))            { purgeHomeDir = f }
