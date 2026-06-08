package tui

import (
	"fmt"
	"os"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyEngram writes the engram MCP server entry into Copilot's mcp-config.json
// (merging, never clobbering other servers), backing the file up only when the
// entry changes, then records the configuration in state. Shared by the configure
// screen and the post-sync re-apply.
func applyEngram(host *copilot.Host, store *state.Store, bkp *backup.Store, rec *state.EngramRecord) error {
	if host == nil || rec == nil {
		return nil
	}
	entry := engram.CopilotCLIEntry(rec.CloudServer)
	want := engram.EntryChecksum(entry)

	cur, ok := engram.CLIEntryChecksum(host.MCPConfigPath)
	if !ok || cur != want {
		if bkp != nil {
			if _, err := os.Stat(host.MCPConfigPath); err == nil {
				if _, err := bkp.CreateFiles("engram", Version, []string{host.MCPConfigPath}); err != nil {
					return fmt.Errorf("backup failed, aborting: %w", err)
				}
			}
		}
		if err := engram.MergeMCPEntry(host.MCPConfigPath, "mcpServers", "engram", entry); err != nil {
			return err
		}
	}

	rec.Checksum = want
	if store != nil {
		return store.SetEngram(rec)
	}
	return nil
}
