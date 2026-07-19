module github.com/runtime-sh/runtime/examples/_generators

go 1.25.0

require (
	github.com/apache/arrow-go/v18 v18.6.0
	github.com/parquet-go/parquet-go v0.30.1
	github.com/runtime-sh/runtime/packages/export v0.0.0
	modernc.org/sqlite v1.53.0
)

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/parquet-go/bitpack v1.0.0 // indirect
	github.com/parquet-go/jsonlite v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/runtime-sh/runtime/packages/datasource v0.0.0 // indirect
	github.com/twpayne/go-geom v1.6.1 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/sys v0.44.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace (
	github.com/runtime-sh/runtime/packages/datasource => ../../packages/datasource
	github.com/runtime-sh/runtime/packages/export => ../../packages/export
)
