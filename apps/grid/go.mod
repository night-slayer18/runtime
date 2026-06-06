module github.com/runtime-sh/runtime/apps/grid

go 1.25.0

require (
	github.com/apache/arrow-go/v18 v18.6.0
	github.com/charmbracelet/bubbles v1.0.0
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/parquet-go/parquet-go v0.30.1
	github.com/runtime-sh/runtime/packages/config v0.0.0
	github.com/runtime-sh/runtime/packages/datasource v0.0.0
	github.com/runtime-sh/runtime/packages/export v0.0.0
	github.com/runtime-sh/runtime/packages/search v0.0.0
	github.com/runtime-sh/runtime/packages/table v0.0.0
	github.com/runtime-sh/runtime/packages/theme v0.0.0
	github.com/runtime-sh/runtime/packages/tui v0.0.0
)

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/colorprofile v0.4.1 // indirect
	github.com/charmbracelet/x/ansi v0.11.6 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.9.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/parquet-go/bitpack v1.0.0 // indirect
	github.com/parquet-go/jsonlite v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/twpayne/go-geom v1.6.1 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	github.com/runtime-sh/runtime/packages/config => ../../packages/config
	github.com/runtime-sh/runtime/packages/datasource => ../../packages/datasource
	github.com/runtime-sh/runtime/packages/export => ../../packages/export
	github.com/runtime-sh/runtime/packages/search => ../../packages/search
	github.com/runtime-sh/runtime/packages/table => ../../packages/table
	github.com/runtime-sh/runtime/packages/theme => ../../packages/theme
	github.com/runtime-sh/runtime/packages/tui => ../../packages/tui
)
