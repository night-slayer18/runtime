package datasource

// registerColumnar registers the columnar binary formats Parquet and Arrow.
//
// Both are now backed by real decoders:
//
//   - Parquet is read with github.com/parquet-go/parquet-go, a pure-Go Parquet
//     implementation (see parquet.go).
//   - Arrow IPC (Feather v2 / .arrow / .ipc) is read with
//     github.com/apache/arrow-go/v18/arrow/ipc (see arrow.go).
//
// They are registered with Available=true, so CheckAvailability passes and Grid
// launches with all five required formats (CSV, TSV, XLSX, Parquet, Arrow)
// genuinely working.
func registerColumnar() {
	registerParquet()
	registerArrow()
}
