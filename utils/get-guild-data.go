package utils

import (
	"github.com/disgoorg/snowflake/v2"
	"github.com/mrmelon54/bigben.mrmelon54.com/value"
)

func GetGuildData(m *map[snowflake.ID]*CacheData, guildId snowflake.ID) *CacheData {
	if a := (*m)[guildId]; a != nil {
		return a
	}
	c := &CacheData{
		User:           map[snowflake.ID]string{},
		TotalClicks:    value.IntValSlice{},
		AvgClickSpeed:  value.FloatValSlice{},
		SlowClickSpeed: value.FloatValSlice{},
		FastClickSpeed: value.FloatValSlice{},
		RawTotalClicks: map[snowflake.ID]int{},
		RawAvgSpeed:    map[snowflake.ID][]float64{},
		RawSlowSpeed:   map[snowflake.ID][]float64{},
		RawFastSpeed:   map[snowflake.ID][]float64{},
	}
	(*m)[guildId] = c
	return c
}
