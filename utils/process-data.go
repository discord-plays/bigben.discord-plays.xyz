package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/MrMelon54/bigben/tables"
	"github.com/disgoorg/snowflake/v2"
	"github.com/mrmelon54/bigben.mrmelon54.com/value"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
)

func ProcessData(cacheDir string, buf2 *bytes.Buffer) error {
	unGzip, err := gzip.NewReader(buf2)
	if err != nil {
		return fmt.Errorf("gzip.NewReader(): %w", err)
	}
	tarReader := tar.NewReader(unGzip)

	guilds := make(map[snowflake.ID]*CacheData)
	users := make(map[snowflake.ID]string)

	log.Println("Reading files from archive")
	for {
		next, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("tarReader.Next(): %w", err)
		}
		switch next.Name {
		case "bong-log.csv":
			err := ReadAllRows[tables.BongLog](tarReader, func(t *tables.BongLog) {
				if !SBool(t.Won) {
					return
				}
				g := GetGuildData(&guilds, t.GuildId)
				g.User[t.UserId] = ""
				g.RawTotalClicks[t.UserId]++
				z := t.InterId.Time().Sub(t.MsgId.Time()).Seconds()

				avg := g.RawAvgSpeed[t.UserId]
				slo := g.RawSlowSpeed[t.UserId]
				fas := g.RawFastSpeed[t.UserId]

				if avg == nil {
					avg = []float64{}
				}
				if slo == nil {
					slo = []float64{}
				}
				if fas == nil {
					fas = []float64{}
				}
				avg = append(avg, z)
				slo = append(slo, z)
				fas = append(fas, z)
				g.RawAvgSpeed[t.UserId] = avg
				g.RawSlowSpeed[t.UserId] = slo
				g.RawFastSpeed[t.UserId] = fas
			})
			if err != nil {
				return err
			}
		case "user-log.csv":
			err := ReadAllRows[tables.UserLog](tarReader, func(t *tables.UserLog) {
				users[t.Id] = t.Tag
			})
			if err != nil {
				return err
			}
		}
	}
	err = unGzip.Close()
	if err != nil {
		return fmt.Errorf("unGzip.Close(): %w", err)
	}

	log.Println("Updating guild user maps")
	for k, v := range guilds {
		for k2 := range v.User {
			v.User[k2] = users[k2]
		}
		v.TotalClicks = func() value.UserStatSlice[int] {
			a := value.UserStatSlice[int]{}
			for k2, v2 := range v.RawTotalClicks {
				a = append(a, value.UserStat[int]{User: k2, Value: v2})
			}
			sort.Sort(sort.Reverse(a))
			return a[:IntMin(a.Len(), 10)]
		}()
		v.AvgClickSpeed = func() value.UserStatSlice[float64] {
			a := value.UserStatSlice[float64]{}
			for k2, v2 := range v.RawAvgSpeed {
				if len(v2) == 0 {
					continue
				}
				var z float64
				for _, i := range v2 {
					z += i
				}
				z = z / float64(len(v2))
				a = append(a, value.UserStat[float64]{User: k2, Value: z})
			}
			sort.Sort(a)
			return a[:IntMin(a.Len(), 10)]
		}()
		v.SlowClickSpeed = func() value.UserStatSlice[float64] {
			a := value.UserStatSlice[float64]{}
			for k2, v2 := range v.RawSlowSpeed {
				if len(v2) == 0 {
					continue
				}
				z := v2[0]
				for _, i := range v2 {
					if i > z {
						z = i
					}
				}
				a = append(a, value.UserStat[float64]{User: k2, Value: z})
			}
			sort.Sort(sort.Reverse(a))
			return a[:IntMin(a.Len(), 10)]
		}()
		v.FastClickSpeed = func() value.UserStatSlice[float64] {
			a := value.UserStatSlice[float64]{}
			for k2, v2 := range v.RawFastSpeed {
				if len(v2) == 0 {
					continue
				}
				z := v2[0]
				for _, i := range v2 {
					if i < z {
						z = i
					}
				}
				a = append(a, value.UserStat[float64]{User: k2, Value: z})
			}
			sort.Sort(a)
			return a[:IntMin(a.Len(), 10)]
		}()

		log.Printf("Creating cache for server %s\n", k.String())
		f, err := os.Create(filepath.Join(cacheDir, k.String()+".json"))
		if err != nil {
			return fmt.Errorf("os.Create('%s.json'): %w", k, err)
		}
		err = json.NewEncoder(f).Encode(v)
		if err != nil {
			return fmt.Errorf("json.Encode(): %w", err)
		}
		err = f.Close()
		if err != nil {
			return fmt.Errorf("f.Close(): %w", err)
		}
	}
	return nil
}
