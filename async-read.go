package main

import (
	"encoding/json"
	"github.com/disgoorg/snowflake/v2"
	"github.com/mrmelon54/bigben.mrmelon54.com/value"
	"os"
	"sync"
)

type AsyncRead struct {
	filename string
	lock     *sync.Mutex
	data     *CacheData
}

func NewAsyncRead(filename string) *AsyncRead {
	return &AsyncRead{filename: filename, lock: &sync.Mutex{}}
}

func (a *AsyncRead) Get() *CacheData {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.data != nil {
		return a.data
	}
	a.data = a.read()
	return a.data
}

func (a *AsyncRead) read() *CacheData {
	fd, err := os.Open(a.filename)
	if err != nil {
		return nil
	}
	var data CacheData
	err = json.NewDecoder(fd).Decode(&data)
	if err != nil {
		return nil
	}
	return &data
}

type CacheData struct {
	User           map[snowflake.ID]string    `json:"user"`
	TotalClicks    value.IntValSlice          `json:"total_clicks"`
	AvgClickSpeed  value.FloatValSlice        `json:"avg_click_speed"`
	SlowClickSpeed value.FloatValSlice        `json:"slow_click_speed"`
	FastClickSpeed value.FloatValSlice        `json:"fast_click_speed"`
	RawTotalClicks map[snowflake.ID]int       `json:"-"`
	RawAvgSpeed    map[snowflake.ID][]float64 `json:"-"`
	RawSlowSpeed   map[snowflake.ID][]float64 `json:"-"`
	RawFastSpeed   map[snowflake.ID][]float64 `json:"-"`
}
