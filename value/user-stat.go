package value

import (
	"cmp"
	"github.com/disgoorg/snowflake/v2"
)

type UserStat[T cmp.Ordered] struct {
	User  snowflake.ID
	Value T
}

type UserStatSlice[T cmp.Ordered] []UserStat[T]

func (s UserStatSlice[T]) Len() int {
	return len(s)
}

func (s UserStatSlice[T]) Less(i, j int) bool {
	return s[i].Value < s[j].Value
}

func (s UserStatSlice[T]) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
