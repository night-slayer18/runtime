package model

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tree"
)

// childByTitle returns the first child whose title equals or starts with the
// given prefix, or nil. Leaf titles render as "key: value", so a prefix match
// on "key" locates a node regardless of its value.
func childByTitlePrefix(n *tree.TreeNode, prefix string) *tree.TreeNode {
	for _, c := range n.Children {
		if c.Title == prefix || len(c.Title) >= len(prefix) && c.Title[:len(prefix)] == prefix {
			return c
		}
	}
	return nil
}

// collectLeaves walks the tree and returns a map of leaf title -> Data for all
// leaf nodes, giving a format-independent view of the document's scalar values.
func collectLeaves(n *tree.TreeNode) map[string]interface{} {
	out := map[string]interface{}{}
	var walk func(node *tree.TreeNode)
	walk = func(node *tree.TreeNode) {
		if node == nil {
			return
		}
		if node.IsLeaf() {
			out[node.Title] = node.Data
			return
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}

func TestParseJSON(t *testing.T) {
	data := []byte(`{"name": "prism", "version": 2, "tags": ["a", "b"], "nested": {"on": true}}`)
	root, err := ParseDocument(data, FormatJSON)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if root.Key != "$" {
		t.Errorf("root key = %q, want $", root.Key)
	}

	name := childByTitlePrefix(root, "name:")
	if name == nil || name.Data != "prism" {
		t.Fatalf("name = %+v, want prism", name)
	}

	tags := childByTitlePrefix(root, "tags")
	if tags == nil || len(tags.Children) != 2 {
		t.Fatalf("tags node = %+v, want 2 children", tags)
	}

	nested := childByTitlePrefix(root, "nested")
	if nested == nil || childByTitlePrefix(nested, "on:") == nil {
		t.Errorf("nested.on not found")
	}
}

func TestParseJSONInvalid(t *testing.T) {
	if _, err := ParseDocument([]byte(`{bad`), FormatJSON); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseXML(t *testing.T) {
	data := []byte(`<config><name>prism</name><server port="8080">local</server></config>`)
	root, err := ParseDocument(data, FormatXML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	cfg := root.Children[0]
	if cfg.Title != "config" {
		t.Fatalf("first element = %q, want config", cfg.Title)
	}
	server := childByTitlePrefix(cfg, "server")
	if server == nil {
		t.Fatal("missing server element")
	}
	if childByTitlePrefix(server, "@port:") == nil {
		t.Errorf("missing port attribute")
	}
	if childByTitlePrefix(server, "#text:") == nil {
		t.Errorf("missing server text node")
	}
}

func TestParseYAML(t *testing.T) {
	data := []byte(`
name: prism
version: 2
enabled: true
tags:
  - a
  - b
nested:
  on: true
  count: 3
servers:
  - host: local
    port: 8080
`)
	root, err := ParseDocument(data, FormatYAML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	if n := childByTitlePrefix(root, "name:"); n == nil || n.Data != "prism" {
		t.Errorf("name = %+v, want prism", n)
	}
	// yaml.v3 decodes integers into Go int when unmarshalling into interface{}.
	if n := childByTitlePrefix(root, "version:"); n == nil || n.Data != int(2) {
		t.Errorf("version = %+v, want 2", n)
	}
	tags := childByTitlePrefix(root, "tags")
	if tags == nil || len(tags.Children) != 2 {
		t.Fatalf("tags = %+v, want 2 children", tags)
	}
	nested := childByTitlePrefix(root, "nested")
	if nested == nil || childByTitlePrefix(nested, "count:") == nil {
		t.Errorf("nested.count missing")
	}
	servers := childByTitlePrefix(root, "servers")
	if servers == nil || len(servers.Children) != 1 {
		t.Fatalf("servers = %+v, want 1 child", servers)
	}
	first := servers.Children[0]
	if childByTitlePrefix(first, "host:") == nil || childByTitlePrefix(first, "port:") == nil {
		t.Errorf("servers[0] missing host/port: %+v", first)
	}
}

func TestParseYAMLInlineCollections(t *testing.T) {
	data := []byte("list: [1, 2, 3]\nmap: {a: 1, b: 2}\n")
	root, err := ParseDocument(data, FormatYAML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	list := childByTitlePrefix(root, "list")
	if list == nil || len(list.Children) != 3 {
		t.Fatalf("list = %+v, want 3 children", list)
	}
	m := childByTitlePrefix(root, "map")
	if m == nil || len(m.Children) != 2 {
		t.Fatalf("map = %+v, want 2 children", m)
	}
}

func TestParseTOML(t *testing.T) {
	data := []byte(`
name = "prism"
version = 2
enabled = true

[server]
host = "local"
port = 8080

[[plugins]]
name = "alpha"

[[plugins]]
name = "beta"
`)
	root, err := ParseDocument(data, FormatTOML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	if n := childByTitlePrefix(root, "name:"); n == nil || n.Data != "prism" {
		t.Errorf("name = %+v, want prism", n)
	}
	if n := childByTitlePrefix(root, "version:"); n == nil || n.Data != int64(2) {
		t.Errorf("version = %+v, want 2", n)
	}
	server := childByTitlePrefix(root, "server")
	if server == nil || childByTitlePrefix(server, "port:") == nil {
		t.Fatalf("server.port missing: %+v", server)
	}
	plugins := childByTitlePrefix(root, "plugins")
	if plugins == nil || len(plugins.Children) != 2 {
		t.Fatalf("plugins = %+v, want 2 children", plugins)
	}
}

func TestParseTOMLDottedKeys(t *testing.T) {
	data := []byte(`a.b.c = 1` + "\n")
	root, err := ParseDocument(data, FormatTOML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	a := childByTitlePrefix(root, "a")
	if a == nil {
		t.Fatal("missing a")
	}
	b := childByTitlePrefix(a, "b")
	if b == nil {
		t.Fatal("missing a.b")
	}
	if childByTitlePrefix(b, "c:") == nil {
		t.Errorf("missing a.b.c")
	}
}

// TestParseYAMLAnchorsAndAliases verifies that anchors (&) and aliases (*),
// which the previous subset parser could not handle, now resolve correctly so
// the aliased node is expanded into a full copy of the anchored value.
func TestParseYAMLAnchorsAndAliases(t *testing.T) {
	data := []byte(`
defaults: &defaults
  adapter: postgres
  host: localhost
development:
  <<: *defaults
  database: dev_db
production:
  <<: *defaults
  database: prod_db
`)
	root, err := ParseDocument(data, FormatYAML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	dev := childByTitlePrefix(root, "development")
	if dev == nil {
		t.Fatal("missing development node")
	}
	// The merge key (<<: *defaults) pulls adapter and host into development.
	if n := childByTitlePrefix(dev, "adapter:"); n == nil || n.Data != "postgres" {
		t.Errorf("development.adapter = %+v, want postgres", n)
	}
	if n := childByTitlePrefix(dev, "host:"); n == nil || n.Data != "localhost" {
		t.Errorf("development.host = %+v, want localhost", n)
	}
	if n := childByTitlePrefix(dev, "database:"); n == nil || n.Data != "dev_db" {
		t.Errorf("development.database = %+v, want dev_db", n)
	}

	prod := childByTitlePrefix(root, "production")
	if prod == nil {
		t.Fatal("missing production node")
	}
	if n := childByTitlePrefix(prod, "adapter:"); n == nil || n.Data != "postgres" {
		t.Errorf("production.adapter = %+v, want postgres", n)
	}
	if n := childByTitlePrefix(prod, "database:"); n == nil || n.Data != "prod_db" {
		t.Errorf("production.database = %+v, want prod_db", n)
	}
}

// TestParseYAMLBlockScalars verifies literal (|) and folded (>) block scalars,
// which the previous subset parser could not handle, now decode into their
// correct multi-line / folded string values.
func TestParseYAMLBlockScalars(t *testing.T) {
	data := []byte("literal: |\n  line one\n  line two\nfolded: >\n  folded line one\n  folded line two\n")
	root, err := ParseDocument(data, FormatYAML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	lit := childByTitlePrefix(root, "literal:")
	if lit == nil || lit.Data != "line one\nline two\n" {
		t.Fatalf("literal = %+v, want %q", lit, "line one\nline two\n")
	}

	folded := childByTitlePrefix(root, "folded:")
	if folded == nil || folded.Data != "folded line one folded line two\n" {
		t.Fatalf("folded = %+v, want %q", folded, "folded line one folded line two\n")
	}
}

// TestParseTOMLMultilineStrings verifies multi-line basic (""") and literal
// (”') strings, which the previous subset parser could not handle, now decode
// correctly.
func TestParseTOMLMultilineStrings(t *testing.T) {
	data := []byte("basic = \"\"\"\nfirst line\nsecond line\n\"\"\"\nliteral = '''\nraw\\nno-escape\n'''\n")
	root, err := ParseDocument(data, FormatTOML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	basic := childByTitlePrefix(root, "basic:")
	if basic == nil || basic.Data != "first line\nsecond line\n" {
		t.Fatalf("basic = %+v, want %q", basic, "first line\nsecond line\n")
	}

	literal := childByTitlePrefix(root, "literal:")
	// In a literal string the backslash sequence is preserved verbatim.
	if literal == nil || literal.Data != "raw\\nno-escape\n" {
		t.Fatalf("literal = %+v, want %q", literal, "raw\\nno-escape\n")
	}
}

// TestParseTOMLDatetimes verifies that offset datetimes, which the previous
// subset parser kept as plain strings, now decode into Go time values and
// render in their canonical RFC 3339 form.
func TestParseTOMLDatetimes(t *testing.T) {
	data := []byte("created = 1979-05-27T07:32:00Z\n")
	root, err := ParseDocument(data, FormatTOML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	created := childByTitlePrefix(root, "created:")
	if created == nil {
		t.Fatalf("missing created node")
	} else if _, ok := created.Data.(time.Time); !ok {
		t.Fatalf("created Data = %T, want time.Time", created.Data)
	} else if got := scalarString(created.Data); got != "1979-05-27T07:32:00Z" {
		t.Errorf("created rendered = %q, want %q", got, "1979-05-27T07:32:00Z")
	}
}

// TestParseTOMLNumericGrammar verifies the full TOML numeric grammar
// (underscore separators and hexadecimal literals), which the previous subset
// parser only partially handled, now decode to the correct int64 values.
func TestParseTOMLNumericGrammar(t *testing.T) {
	data := []byte("big = 1_000_000\nhex = 0xDEAD_BEEF\noct = 0o755\nbin = 0b1010\n")
	root, err := ParseDocument(data, FormatTOML)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}

	cases := map[string]int64{
		"big:": 1000000,
		"hex:": 0xDEADBEEF,
		"oct:": 0o755,
		"bin:": 0b1010,
	}
	for prefix, want := range cases {
		n := childByTitlePrefix(root, prefix)
		if n == nil || n.Data != want {
			t.Errorf("%s = %+v, want %d", prefix, n, want)
		}
	}
}

// TestEquivalentTreesAcrossFormats verifies that the same logical document
// expressed in JSON, YAML, and TOML parses into equivalent leaf sets. This
// exercises the requirement that each supported format parses into an
// equivalent tree.
func TestEquivalentTreesAcrossFormats(t *testing.T) {
	jsonDoc := []byte(`{"name": "prism", "version": 2, "enabled": true}`)
	yamlDoc := []byte("name: prism\nversion: 2\nenabled: true\n")
	tomlDoc := []byte("name = \"prism\"\nversion = 2\nenabled = true\n")

	// JSON numbers decode to float64; per-format expectations are set below.

	cases := []struct {
		name   string
		data   []byte
		format Format
		verNum interface{}
	}{
		{"json", jsonDoc, FormatJSON, float64(2)},
		{"yaml", yamlDoc, FormatYAML, int(2)},
		{"toml", tomlDoc, FormatTOML, int64(2)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ParseDocument(tc.data, tc.format)
			if err != nil {
				t.Fatalf("ParseDocument: %v", err)
			}
			leaves := collectLeaves(root)

			titles := make([]string, 0, len(leaves))
			for k := range leaves {
				titles = append(titles, k)
			}
			sort.Strings(titles)
			wantTitles := []string{"enabled: true", "name: prism", "version: 2"}
			if !reflect.DeepEqual(titles, wantTitles) {
				t.Fatalf("leaf titles = %v, want %v", titles, wantTitles)
			}

			if leaves["name: prism"] != "prism" {
				t.Errorf("name = %v", leaves["name: prism"])
			}
			if leaves["enabled: true"] != true {
				t.Errorf("enabled = %v", leaves["enabled: true"])
			}
			if leaves["version: 2"] != tc.verNum {
				t.Errorf("version = %v (%T), want %v", leaves["version: 2"], leaves["version: 2"], tc.verNum)
			}
		})
	}
}

func TestFormatFromPath(t *testing.T) {
	cases := map[string]struct {
		want Format
		ok   bool
	}{
		"a.json": {FormatJSON, true},
		"a.yaml": {FormatYAML, true},
		"a.yml":  {FormatYAML, true},
		"a.toml": {FormatTOML, true},
		"a.xml":  {FormatXML, true},
		"a.txt":  {"", false},
		"noext":  {"", false},
	}
	for path, want := range cases {
		got, ok := FormatFromPath(path)
		if got != want.want || ok != want.ok {
			t.Errorf("FormatFromPath(%q) = (%q, %v), want (%q, %v)", path, got, ok, want.want, want.ok)
		}
	}
}

func TestModelSearchLocatesNodes(t *testing.T) {
	m := New(theme.DefaultStyles)
	data := []byte(`{"name": "prism", "version": 2, "namespace": "tools"}`)
	root, err := ParseDocument(data, FormatJSON)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	m.LoadDocument(root, FormatJSON)

	// Search by key: both "name" and "namespace" contain "name".
	if n := m.Search("name"); n != 2 {
		t.Errorf("Search(name) = %d, want 2", n)
	}
	// The selection should move to the first match.
	if sel := m.Document().Selected(); sel == nil {
		t.Fatal("no node selected after search")
	}

	// Search by value: "prism" appears in the name node's label.
	if n := m.Search("prism"); n != 1 {
		t.Errorf("Search(prism) = %d, want 1", n)
	}
	sel := m.Document().Selected()
	if sel == nil || sel.Data != "prism" {
		t.Errorf("selected = %+v, want node with value prism", sel)
	}

	// Empty query clears matches.
	if n := m.Search(""); n != 0 {
		t.Errorf("Search(\"\") = %d, want 0", n)
	}
}

func TestModelLoadFileUnsupported(t *testing.T) {
	m := New(theme.DefaultStyles)
	if err := m.LoadFile("/tmp/does-not-exist.txt"); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}
