package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadCommandDir parses every *.yaml file in dir (sorted by filename for
// deterministic registration order) into a fresh CommandRegistry. Parsing is
// strict: any invalid definition aborts the whole load and returns the
// accumulated errors — configuration is never silently discarded.
// A missing directory returns an empty registry and no error, so deployments
// without command definitions need no special-casing.
func LoadCommandDir(dir string) (*CommandRegistry, error) {
	reg := NewCommandRegistry()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return reg, fmt.Errorf("command dir %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var errs []error
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		def, err := ParseCommandResource(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if err := reg.Register(def); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
		}
	}
	if len(errs) > 0 {
		return reg, fmt.Errorf("command definitions invalid (%d errors): %w", len(errs), errs[0])
	}
	return reg, nil
}
