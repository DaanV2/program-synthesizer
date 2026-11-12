package project

import (
	"bytes"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SaveKeepingComments is like Save, but for spec.yaml it carries comments
// and flow/block/quoting style over from the file on disk to the keys that
// still exist after the change. Comments attached to removed keys are
// dropped, and blank lines are not kept (the YAML library does not record
// them). spec.json has no comments, so it is written as by Save.
func (p *Project) SaveKeepingComments() error {
	if strings.HasSuffix(p.SpecPath, ".json") {
		return p.Save()
	}
	old, err := os.ReadFile(p.SpecPath)
	if err != nil {
		return p.Save()
	}
	if err := p.Spec.Validate(); err != nil {
		return err
	}
	b, err := MergeYAMLComments(old, p.Spec)
	if err != nil {
		return p.Save()
	}

	return writeFileAtomic(p.SpecPath, b)
}

// MergeYAMLComments encodes s as YAML and copies comments and node styles
// from the old document onto nodes found at the same key path.
func MergeYAMLComments(old []byte, s *Spec) ([]byte, error) {
	fresh, err := s.EncodeYAML()
	if err != nil {
		return nil, err
	}
	var oldDoc, newDoc yaml.Node
	if err := yaml.Unmarshal(old, &oldDoc); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(fresh, &newDoc); err != nil {
		return nil, err
	}
	mergeNode(&oldDoc, &newDoc)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&newDoc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func copyComments(from, to *yaml.Node) {
	to.HeadComment = from.HeadComment
	to.LineComment = from.LineComment
	to.FootComment = from.FootComment
}

func mergeNode(old, nw *yaml.Node) {
	if old == nil || nw == nil {
		return
	}
	if old.Kind != nw.Kind {
		return // e.g. a scalar type became a record: its comments may not fit
	}
	copyComments(old, nw)
	switch nw.Kind {
	case yaml.DocumentNode:
		if len(old.Content) == 1 && len(nw.Content) == 1 {
			mergeNode(old.Content[0], nw.Content[0])
		}
	case yaml.MappingNode:
		nw.Style |= old.Style & yaml.FlowStyle
		for i := 0; i+1 < len(nw.Content); i += 2 {
			key := nw.Content[i].Value
			for j := 0; j+1 < len(old.Content); j += 2 {
				if old.Content[j].Value == key {
					copyComments(old.Content[j], nw.Content[i])
					mergeNode(old.Content[j+1], nw.Content[i+1])

					break
				}
			}
		}
	case yaml.SequenceNode:
		nw.Style = old.Style
		for i := range min(len(old.Content), len(nw.Content)) {
			mergeNode(old.Content[i], nw.Content[i])
		}
	case yaml.ScalarNode:
		// Keep quoting or block style only while the value is unchanged in
		// kind; plain (0) lets the encoder choose.
		if old.Tag == nw.Tag && old.Style != 0 {
			nw.Style = old.Style
		}
	case yaml.AliasNode: // the encoder never produces aliases
	}
}
