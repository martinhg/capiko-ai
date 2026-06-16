package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
)

const advisoryTimeout = 2 * time.Second

type advisoryMsg struct{ text string }

// checkAdvisoryCmd fetches the remote advisory off the UI thread. Any failure
// is silently swallowed — the advisory is purely informational and fail-open.
func checkAdvisoryCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), advisoryTimeout)
		defer cancel()
		return advisoryMsg{text: release.Advisory(ctx)}
	}
}
