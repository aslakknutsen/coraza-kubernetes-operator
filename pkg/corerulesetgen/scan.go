package corerulesetgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanResult lists discovered rule and data files under a rules directory.
type ScanResult struct {
	ConfPaths      []string
	DataPaths      []string
	PMFromFileRefs bool
}

// Scan globs and sorts *.conf and *.data under rulesPath and detects @pmFromFile
// in any .conf file. rulesPath must exist and be a directory (caller validates).
func Scan(rulesPath string) (ScanResult, error) {
	confFiles, err := filepath.Glob(filepath.Join(rulesPath, "*.conf"))
	if err != nil {
		return ScanResult{}, err
	}
	sort.Strings(confFiles)

	dataFiles, err := filepath.Glob(filepath.Join(rulesPath, "*.data"))
	if err != nil {
		return ScanResult{}, err
	}
	sort.Strings(dataFiles)

	var pmFromFileRefs bool
	for _, p := range confFiles {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return ScanResult{}, fmt.Errorf("read %s: %w", p, rerr)
		}
		if strings.Contains(string(raw), "@pmFromFile") {
			pmFromFileRefs = true
			break
		}
	}
	return ScanResult{
		ConfPaths:      confFiles,
		DataPaths:      dataFiles,
		PMFromFileRefs: pmFromFileRefs,
	}, nil
}
