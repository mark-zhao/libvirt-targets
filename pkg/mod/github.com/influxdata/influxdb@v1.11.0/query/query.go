package query // import "github.com/influxdata/influxdb/query"

//go:generate tmpl -data=@tmpldata iterator.gen.go.tmpl
//go:generate tmpl -data=@tmpldata point.gen.go.tmpl
//go:generate tmpl -data=@tmpldata functions.gen.go.tmpl

//go:generate protoc --go_out=./internal internal/internal.proto
