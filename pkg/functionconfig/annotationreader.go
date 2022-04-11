package functionconfig

import "time"

type DurationConfigField struct {
	Name    string
	Value   string
	Field   *time.Duration
	Default time.Duration
}

type AnnotationConfigField struct {
	Key             string
	ValueString     *string
	ValueListString []string
	ValueInt        *int
	ValueUInt64     *uint64
	ValueBool       *bool
}
