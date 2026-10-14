package main

import (
	"fmt"
	"io"
	"time"

	"github.com/martinhg/capiko-ai/internal/sessionlog"
	"github.com/martinhg/capiko-ai/internal/sessionreport"
)

const sinceLayout = "2006-01-02"

type reportOpts struct {
	sessionID string
	all       bool
	since     time.Time
	hasSince  bool
	asJSON    bool
}

// reportCommand renders a report from the captured session logs.
//
//	capiko-ai report [--session ID | --all] [--since YYYY-MM-DD] [--json]
//
// --since without --session aggregates across sessions, like --all.
func reportCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "report" {
		return false, 0, nil
	}
	opts, err := parseReportArgs(args)
	if err != nil {
		return true, 1, err
	}

	home, err := userHomeDirFn()
	if err != nil {
		return true, 1, fmt.Errorf("report: resolve home directory: %w", err)
	}
	dir := sessionlog.Dir(home)

	var text string
	switch {
	case opts.sessionID != "":
		events, err := sessionlog.ReadSession(dir, opts.sessionID)
		if err != nil {
			return true, 1, fmt.Errorf("report: %w", err)
		}
		r := sessionreport.Analyze(filterSince(events, opts))
		r.SessionID = opts.sessionID
		text, err = renderSingle(r, opts.asJSON)
		if err != nil {
			return true, 1, err
		}
	case opts.all || opts.hasSince:
		all, err := sessionlog.ReadAll(dir)
		if err != nil {
			return true, 1, fmt.Errorf("report: %w", err)
		}
		filtered := map[string][]sessionlog.Event{}
		for id, events := range all {
			if ev := filterSince(events, opts); len(ev) > 0 {
				filtered[id] = ev
			}
		}
		if len(filtered) == 0 {
			return true, 1, fmt.Errorf("report: no sessions found in %s", dir)
		}
		agg := sessionreport.AnalyzeAll(filtered)
		if opts.asJSON {
			text, err = sessionreport.RenderAggregateJSON(agg)
		} else {
			text = sessionreport.RenderAggregateText(agg)
		}
		if err != nil {
			return true, 1, err
		}
	default:
		id, events, err := latestSession(dir)
		if err != nil {
			return true, 1, err
		}
		r := sessionreport.Analyze(events)
		r.SessionID = id
		text, err = renderSingle(r, opts.asJSON)
		if err != nil {
			return true, 1, err
		}
	}

	fmt.Fprint(out, text)
	return true, 0, nil
}

func renderSingle(r sessionreport.Report, asJSON bool) (string, error) {
	if asJSON {
		return sessionreport.RenderJSON(r)
	}
	return sessionreport.RenderText(r), nil
}

// latestSession picks the session whose last parseable event is newest; ties
// and sessions without timestamps fall back to the highest session ID.
func latestSession(dir string) (string, []sessionlog.Event, error) {
	all, err := sessionlog.ReadAll(dir)
	if err != nil {
		return "", nil, fmt.Errorf("report: %w", err)
	}
	var bestID string
	var bestTS time.Time
	for id, events := range all {
		ts := lastTimestamp(events)
		if bestID == "" || ts.After(bestTS) || (ts.Equal(bestTS) && id > bestID) {
			bestID, bestTS = id, ts
		}
	}
	if bestID == "" {
		return "", nil, fmt.Errorf("report: no sessions found in %s", dir)
	}
	return bestID, all[bestID], nil
}

func lastTimestamp(events []sessionlog.Event) time.Time {
	var last time.Time
	for _, ev := range events {
		if t, err := time.Parse(time.RFC3339, ev.TS); err == nil && t.After(last) {
			last = t
		}
	}
	return last
}

// filterSince keeps events at or after the --since date. Events with an
// unparseable timestamp are dropped when a filter is active.
func filterSince(events []sessionlog.Event, opts reportOpts) []sessionlog.Event {
	if !opts.hasSince {
		return events
	}
	var kept []sessionlog.Event
	for _, ev := range events {
		if t, err := time.Parse(time.RFC3339, ev.TS); err == nil && !t.Before(opts.since) {
			kept = append(kept, ev)
		}
	}
	return kept
}

func parseReportArgs(args []string) (reportOpts, error) {
	var o reportOpts
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--json":
			o.asJSON = true
		case "--all":
			o.all = true
		case "--session", "--since":
			if i+1 >= len(args) {
				return o, fmt.Errorf("report: %s requires a value", a)
			}
			i++
			if a == "--session" {
				o.sessionID = args[i]
				continue
			}
			t, err := time.Parse(sinceLayout, args[i])
			if err != nil {
				return o, fmt.Errorf("report: invalid --since date %q (want YYYY-MM-DD)", args[i])
			}
			o.since, o.hasSince = t, true
		default:
			return o, fmt.Errorf("report: unknown argument %q", a)
		}
	}
	if o.sessionID != "" && o.all {
		return o, fmt.Errorf("report: --session and --all are mutually exclusive")
	}
	return o, nil
}
