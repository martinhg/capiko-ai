package skill

import (
	"fmt"
	"sort"
)

// ValidateDependencies checks that every skill's depends_on names a skill that
// exists in the catalog and that the dependency graph has no cycles. It is meant
// to run once at catalog load time: a violation is an authoring error, not a
// user error, so the caller surfaces it as a hard failure.
func ValidateDependencies(catalog []Skill) error {
	byName := make(map[string]Skill, len(catalog))
	for _, s := range catalog {
		byName[s.Name] = s
	}

	// Every declared dependency must resolve to a catalog entry.
	for _, s := range catalog {
		for _, dep := range s.DependsOn {
			if _, ok := byName[dep]; !ok {
				return fmt.Errorf("skill %q depends on %q, which is not in the catalog", s.Name, dep)
			}
		}
	}

	// Detect cycles with a three-color DFS (white=unvisited, gray=on stack,
	// black=done). Hitting a gray node means we looped back into the stack.
	const (
		white = iota
		gray
		black
	)
	color := make(map[string]int, len(catalog))
	var visit func(name string) error
	visit = func(name string) error {
		color[name] = gray
		for _, dep := range byName[name].DependsOn {
			switch color[dep] {
			case gray:
				return fmt.Errorf("dependency cycle detected through %q", dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}
	for _, s := range catalog {
		if color[s.Name] == white {
			if err := visit(s.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResolveDependencies expands a set of selected skill names into the full set
// that must be installed together: the selection plus every transitive
// dependency. The result is sorted and deduplicated. Every selected name must
// exist in the catalog; a missing dependency in the graph is also an error.
// Cycles are tolerated here (each node is visited once) — ValidateDependencies
// is the place that rejects them.
func ResolveDependencies(catalog []Skill, selected []string) ([]string, error) {
	byName := make(map[string]Skill, len(catalog))
	for _, s := range catalog {
		byName[s.Name] = s
	}

	resolved := make(map[string]bool)
	var add func(name string) error
	add = func(name string) error {
		if resolved[name] {
			return nil
		}
		s, ok := byName[name]
		if !ok {
			return fmt.Errorf("skill %q is not in the catalog", name)
		}
		resolved[name] = true
		for _, dep := range s.DependsOn {
			if err := add(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range selected {
		if err := add(name); err != nil {
			return nil, err
		}
	}

	out := make([]string, 0, len(resolved))
	for name := range resolved {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
