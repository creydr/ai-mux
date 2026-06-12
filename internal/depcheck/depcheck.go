package depcheck

import (
	"fmt"
	"os/exec"
	"strings"
)

type Dependency struct {
	Name   string
	Reason string
}

func Check(deps []Dependency) error {
	var missing []string
	for _, dep := range deps {
		if _, err := exec.LookPath(dep.Name); err != nil {
			missing = append(missing, fmt.Sprintf("  - %s (%s)", dep.Name, dep.Reason))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required tools:\n%s", strings.Join(missing, "\n"))
	}
	return nil
}
