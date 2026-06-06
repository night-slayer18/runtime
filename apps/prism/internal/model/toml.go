package model

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/runtime-sh/runtime/packages/tree"
)

// parseTOML parses TOML into a tree using the full-spec
// github.com/pelletier/go-toml/v2 library.
//
// The document is unmarshalled into a map[string]interface{} and handed to the
// shared valueToNode converter. Using the real library provides full TOML 1.0
// compliance, including multi-line basic and literal strings, the complete
// numeric grammar (underscores, hex/octal/binary, inf/nan), offset and local
// datetimes, arrays of tables, dotted keys, and inline tables. Datetimes
// decode into Go time types, which valueToNode renders via their string form.
func parseTOML(data []byte) (*tree.TreeNode, error) {
	root := map[string]interface{}{}
	if err := toml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	return valueToNode("$", "root", root), nil
}
