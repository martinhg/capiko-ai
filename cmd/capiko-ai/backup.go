package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
)

type backupInputs struct {
	host         *copilot.Host
	hostExitCode int
	hostErr      error
	bkp          *backup.Store
}

var gatherBackupInputs = func() (backupInputs, error) {
	host, exitCode, hostErr := requireHost(copilot.Detect)
	in := backupInputs{host: host, hostExitCode: exitCode, hostErr: hostErr}
	if exitCode != 0 {
		return in, nil
	}
	bkp, err := backup.DefaultStore()
	if err != nil {
		return in, err
	}
	in.bkp = bkp
	return in, nil
}

// backupCommand handles headless backup operations.
//
//	capiko-ai backup list [--json]
//	capiko-ai backup restore --id <id>
func backupCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "backup" {
		return false, 0, nil
	}

	if len(args) == 0 {
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  capiko-ai backup list [--json]")
		fmt.Fprintln(out, "  capiko-ai backup restore --id <id>")
		return true, 1, fmt.Errorf("backup: subcommand required (list or restore)")
	}

	in, err := gatherBackupInputs()
	if err != nil {
		return true, 1, err
	}
	if in.hostExitCode != 0 {
		msg := "GitHub Copilot CLI not found"
		if in.hostExitCode == 1 && in.hostErr != nil {
			msg = in.hostErr.Error()
		}
		fmt.Fprintln(out, msg)
		return true, in.hostExitCode, in.hostErr
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list":
		return backupList(in, subArgs, out)
	case "restore":
		return backupRestore(in, subArgs, out)
	default:
		fmt.Fprintf(out, "backup: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  capiko-ai backup list [--json]")
		fmt.Fprintln(out, "  capiko-ai backup restore --id <id>")
		return true, 1, nil
	}
}

func backupList(in backupInputs, args []string, out io.Writer) (bool, int, error) {
	var asJSON bool
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(out, "backup list: unknown argument %q\n", a)
			return true, 1, nil
		}
	}

	manifests, err := in.bkp.List()
	if err != nil {
		fmt.Fprintf(out, "backup list: %v\n", err)
		return true, 1, nil
	}

	if asJSON {
		if manifests == nil {
			manifests = []backup.Manifest{}
		}
		b, err := json.MarshalIndent(manifests, "", "  ")
		if err != nil {
			return true, 1, err
		}
		fmt.Fprintln(out, string(b))
		return true, 0, nil
	}

	if len(manifests) == 0 {
		fmt.Fprintln(out, "No backups found.")
		return true, 0, nil
	}

	fmt.Fprintf(out, "%d backup(s)\n\n", len(manifests))
	for _, m := range manifests {
		skills := len(m.Entries)
		files := len(m.Files)
		fmt.Fprintf(out, "  %s  %-10s  v%-8s  %d skill(s), %d file(s)  %s\n",
			m.ID, m.Reason, m.Version, skills, files,
			m.CreatedAt.Format("2006-01-02 15:04"))
	}
	return true, 0, nil
}

func backupRestore(in backupInputs, args []string, out io.Writer) (bool, int, error) {
	var id string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 >= len(args) {
				fmt.Fprintln(out, "backup restore: --id requires a value")
				return true, 1, nil
			}
			i++
			id = args[i]
		default:
			fmt.Fprintf(out, "backup restore: unknown argument %q\n", args[i])
			return true, 1, nil
		}
	}

	if id == "" {
		fmt.Fprintln(out, "backup restore: --id <id> is required")
		fmt.Fprintln(out, "Run 'capiko-ai backup list' to see available backup IDs.")
		return true, 1, nil
	}

	if err := in.bkp.Restore(in.host.SkillsDir, id); err != nil {
		fmt.Fprintf(out, "backup restore: %v\n", err)
		return true, 1, nil
	}

	fmt.Fprintf(out, "Restored backup %s\n", id)
	return true, 0, nil
}
