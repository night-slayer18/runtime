// Package model defines the core data model for prism: parsing structured
// documents (JSON, YAML, TOML, XML) into a shared tree.TreeNode hierarchy and
// holding the application state that the UI layer renders and navigates.
package model

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/runtime-sh/runtime/packages/tree"
)

// Format identifies a supported structured-document format.
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatTOML Format = "toml"
	FormatXML  Format = "xml"
)

// FormatFromPath maps a file extension to a supported Format. It returns the
// detected format and true on success, or the zero Format and false when the
// extension is not recognised.
func FormatFromPath(path string) (Format, bool) {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "json":
		return FormatJSON, true
	case "yaml", "yml":
		return FormatYAML, true
	case "toml":
		return FormatTOML, true
	case "xml":
		return FormatXML, true
	default:
		return "", false
	}
}

// ParseDocument parses data in the given format into a tree.TreeNode hierarchy.
// The returned root is suitable for installation into a *tree.Tree via SetRoot.
func ParseDocument(data []byte, format Format) (*tree.TreeNode, error) {
	switch format {
	case FormatJSON:
		return parseJSON(data)
	case FormatYAML:
		return parseYAML(data)
	case FormatTOML:
		return parseTOML(data)
	case FormatXML:
		return parseXML(data)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

// parseJSON decodes JSON using the standard library and converts the resulting
// value into a tree rooted at "$".
func parseJSON(data []byte) (*tree.TreeNode, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return valueToNode("$", "root", v), nil
}

// valueToNode converts an arbitrary decoded value (maps, slices, scalars) into
// a tree node. key is the unique, path-qualified key used for navigation and
// expand/collapse state; title is the human-facing label shown for the node.
//
// Container nodes (maps and slices) carry no Data payload; leaf nodes store the
// scalar value in Data and render it inline in the title so users see the value
// without descending. Map keys are sorted for deterministic output.
func valueToNode(key, title string, v interface{}) *tree.TreeNode {
	switch val := v.(type) {
	case map[string]interface{}:
		node := &tree.TreeNode{Key: key, Title: title}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := valueToNode(key+"."+k, k, val[k])
			node.Children = append(node.Children, child)
		}
		return node
	case map[interface{}]interface{}:
		// YAML maps may be keyed by non-string scalars; normalise to strings.
		norm := make(map[string]interface{}, len(val))
		for k, vv := range val {
			norm[fmt.Sprintf("%v", k)] = vv
		}
		return valueToNode(key, title, norm)
	case []interface{}:
		node := &tree.TreeNode{Key: key, Title: title}
		for i, item := range val {
			child := valueToNode(fmt.Sprintf("%s[%d]", key, i), fmt.Sprintf("[%d]", i), item)
			node.Children = append(node.Children, child)
		}
		return node
	default:
		return &tree.TreeNode{
			Key:   key,
			Title: fmt.Sprintf("%s: %s", title, scalarString(v)),
			Data:  v,
		}
	}
}

// scalarString renders a scalar value for inline display in a node title.
func scalarString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case string:
		return val
	case float64:
		// Render integral float64 values (the JSON number default) without a
		// trailing ".0" so 3 shows as "3" rather than "3".
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case time.Time:
		// go-toml/v2 decodes offset datetimes into time.Time; render in the
		// canonical RFC 3339 form.
		return val.Format(time.RFC3339)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}
