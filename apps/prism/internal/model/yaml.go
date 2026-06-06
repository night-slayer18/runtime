package model

import (
	"fmt"

	"github.com/runtime-sh/runtime/packages/tree"
	"gopkg.in/yaml.v3"
)

// parseYAML parses YAML into a tree using the full-spec gopkg.in/yaml.v3
// library.
//
// The document is unmarshalled into an interface{} and handed to the shared
// valueToNode converter. yaml.v3 decodes mappings with string keys into
// map[string]interface{} and mappings with non-string keys into
// map[interface{}]interface{}; valueToNode normalises both. Supporting the
// real library means the full YAML 1.2 feature set is available, including
// block scalars (| and >), anchors/aliases, flow collections, multiple-line
// plain scalars, and tagged values.
func parseYAML(data []byte) (*tree.TreeNode, error) {
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return valueToNode("$", "root", v), nil
}
