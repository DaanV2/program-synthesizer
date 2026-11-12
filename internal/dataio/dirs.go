package dataio

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"synth/internal/project"
)

// ReadDirs pairs the *.json files in inDir and outDir by basename. Each pair
// becomes a Record whose Name is the shared basename (Plan replaces it with
// a generated name if it is not a valid example name). Files present on
// only one side are reported as warnings. Subdirectories are ignored.
func ReadDirs(inDir, outDir string) (*Batch, error) {
	ins, err := jsonFiles(inDir)
	if err != nil {
		return nil, err
	}
	outs, err := jsonFiles(outDir)
	if err != nil {
		return nil, err
	}
	if same, _ := sameDir(inDir, outDir); same {
		return nil, fmt.Errorf("--inputs and --outputs are the same directory (%s)", inDir)
	}

	b := &Batch{}
	names := make([]string, 0, len(ins))
	for name := range ins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		inPath := ins[name]
		outPath, ok := outs[name]
		if !ok {
			b.Warnings = append(b.Warnings, fmt.Sprintf("%s: no matching output in %s, skipped", inPath, outDir))
			continue
		}
		in, errIn := project.ReadValueFile(inPath)
		out, errOut := project.ReadValueFile(outPath)
		if errIn != nil || errOut != nil {
			for _, e := range []error{errIn, errOut} {
				if e != nil {
					b.Problems = append(b.Problems, e)
				}
			}
			continue
		}
		b.Records = append(b.Records, Record{Source: inPath, Name: name, Input: in, Output: out})
	}
	var orphans []string
	for name, outPath := range outs {
		if _, ok := ins[name]; !ok {
			orphans = append(orphans, fmt.Sprintf("%s: no matching input in %s, skipped", outPath, inDir))
		}
	}
	sort.Strings(orphans)
	b.Warnings = append(b.Warnings, orphans...)
	return b, nil
}

// jsonFiles maps basename (without extension) to path for every regular
// *.json file in dir. The extension is matched case-insensitively so that
// FOO.JSON from Windows tools is picked up too.
func jsonFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || !strings.EqualFold(ext, project.Ext) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ext)
		if prev, dup := files[name]; dup {
			return nil, fmt.Errorf("two files with basename %q in %s (%s and %s)", name, dir, filepath.Base(prev), e.Name())
		}
		files[name] = filepath.Join(dir, e.Name())
	}
	return files, nil
}

func sameDir(a, b string) (bool, error) {
	sa, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(sa, sb), nil
}
