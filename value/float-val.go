package value

import "github.com/disgoorg/snowflake/v2"

type FloatVal struct {
	User  snowflake.ID
	Value float64
}

type FloatValSlice []FloatVal

func (s FloatValSlice) Len() int {
	return len(s)
}

func (s FloatValSlice) Less(i, j int) bool {
	return s[i].Value < s[j].Value
}

func (s FloatValSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
