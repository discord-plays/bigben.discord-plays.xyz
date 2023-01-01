package value

import "github.com/disgoorg/snowflake/v2"

type IntVal struct {
	User  snowflake.ID
	Value int
}

type IntValSlice []IntVal

func (s IntValSlice) Len() int {
	return len(s)
}

func (s IntValSlice) Less(i, j int) bool {
	return s[i].Value < s[j].Value
}

func (s IntValSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
