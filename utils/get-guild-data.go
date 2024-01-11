package utils

import (
	"github.com/discord-plays/bigben.discord-plays.xyz/value"
	"github.com/disgoorg/snowflake/v2"
)

func GetGuildData(m *map[snowflake.ID]*CacheData, guildId snowflake.ID) *CacheData {
	if a := (*m)[guildId]; a != nil {
		return a
	}
	c := &CacheData{
		User:           map[snowflake.ID]string{},
		TotalClicks:    value.UserStatSlice[int]{},
		AvgClickSpeed:  value.UserStatSlice[float64]{},
		SlowClickSpeed: value.UserStatSlice[float64]{},
		FastClickSpeed: value.UserStatSlice[float64]{},
		RawTotalClicks: map[snowflake.ID]int{},
		RawAvgSpeed:    map[snowflake.ID][]float64{},
		RawSlowSpeed:   map[snowflake.ID][]float64{},
		RawFastSpeed:   map[snowflake.ID][]float64{},
	}
	(*m)[guildId] = c
	return c
}
