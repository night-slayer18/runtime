package model

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/runtime-sh/runtime/packages/tree"
)

// parseXML decodes XML using the standard library's streaming decoder and
// builds a tree that mirrors the element hierarchy. Each element becomes a
// node; its attributes become "@name" leaf children and any non-whitespace
// character data becomes a "#text" leaf child. This preserves document order
// and supports arbitrarily nested structures.
func parseXML(data []byte) (*tree.TreeNode, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))

	root := &tree.TreeNode{Key: "$", Title: "root"}
	// stack tracks the chain of open elements; the last entry is the current
	// parent that new tokens attach to.
	stack := []*tree.TreeNode{root}
	// counts assigns a per-parent occurrence index to each child element so
	// sibling keys stay unique even when element names repeat.
	counts := map[*tree.TreeNode]int{}

	seenElement := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			seenElement = true
			parent := stack[len(stack)-1]
			idx := counts[parent]
			counts[parent] = idx + 1

			node := &tree.TreeNode{
				Key:   fmt.Sprintf("%s/%s[%d]", parent.Key, t.Name.Local, idx),
				Title: t.Name.Local,
			}
			for _, attr := range t.Attr {
				node.Children = append(node.Children, &tree.TreeNode{
					Key:   fmt.Sprintf("%s/@%s", node.Key, attr.Name.Local),
					Title: fmt.Sprintf("@%s: %s", attr.Name.Local, attr.Value),
					Data:  attr.Value,
				})
			}
			parent.Children = append(parent.Children, node)
			stack = append(stack, node)

		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text == "" {
				continue
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, &tree.TreeNode{
				Key:   parent.Key + "/#text",
				Title: fmt.Sprintf("#text: %s", text),
				Data:  text,
			})
		}
	}

	if !seenElement {
		return nil, fmt.Errorf("invalid XML: no elements found")
	}
	return root, nil
}
