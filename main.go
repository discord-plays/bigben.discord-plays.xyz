package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/MrMelon54/bigben/tables"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gocarina/gocsv"
	"github.com/gorilla/mux"
	"github.com/mrmelon54/bigben.mrmelon54.com/value"
	"gopkg.in/yaml.v3"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	//go:embed www/home.go.html
	rawHomePage string
	//go:embed www/leaderboard.go.html
	rawLeaderboardPage string
	//go:embed www/assets/bigben.png
	rawLogo []byte

	homePage = func() *template.Template {
		parse, err := template.New("home").Parse(rawHomePage)
		if err != nil {
			log.Fatal(err)
		}
		return parse
	}()
	leaderboardPage = func() *template.Template {
		parse, err := template.New("leaderboard").Parse(rawLeaderboardPage)
		if err != nil {
			log.Fatal(err)
		}
		return parse
	}()
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "conf", "config.yml", "Config file")
	flag.Parse()

	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatal("os.Open(): ", err)
	}
	var config Config
	decoder := yaml.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("yaml.Decode(): ", err)
	}

	err = os.MkdirAll(config.Cache, os.ModePerm)
	if err != nil {
		log.Fatal("os.MkdirAll(): ", err)
	}

	log.Println("Building data cache")
	yearMap := make(map[string]*yearItem)
	for k, v := range config.Year {
		yp := filepath.Join(config.Cache, k)
		yearMap[k] = &yearItem{
			lock:        &sync.RWMutex{},
			downloadUrl: v,
			dir:         yp,
			guilds:      make(map[string]*AsyncRead),
		}
		if _, err := os.Stat(yp); errors.Is(err, os.ErrNotExist) {
			log.Printf("Cache missing for %s, downloading...\n", k)
			err = downloadData(yp, v)
			if err != nil {
				log.Fatal("Failed to download and process data: ", err)
			}
		}
	}

	router := mux.NewRouter()
	router.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		_ = homePage.Execute(rw, nil)

	})
	router.HandleFunc("/bigben.png", func(rw http.ResponseWriter, req *http.Request) {
		http.ServeContent(rw, req, "bigben.png", time.Now(), bytes.NewReader(rawLogo))
	})
	router.HandleFunc("/{year}/{guild}", func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		yearNum := vars["year"]
		guildId := vars["guild"]
		if y, ok := yearMap[yearNum]; ok {
			if y.lock.TryRLock() {
				y2 := y.guilds[guildId]
				y.lock.RUnlock()
				if y2 == nil {
					y.lock.Lock()
					y.guilds[guildId] = NewAsyncRead(y.dir)
					y.lock.Unlock()
				}
				c := y2.Get()
				_ = leaderboardPage.Execute(rw, c)
				return
			}
			http.Error(rw, "404 Not Found", http.StatusNotFound)
			return
		}
		http.Error(rw, "404 Not Found", http.StatusNotFound)
	}).Methods(http.MethodGet)

	log.Printf("Serving HTTP on '%s'\n", config.Listen)
	srv := &http.Server{
		Addr:    config.Listen,
		Handler: router,
	}
	log.Fatal("ListenAndServe(): ", srv.ListenAndServe())
}

func downloadData(cacheDir, downloadUrl string) error {
	err := os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("os.MkdirAll(): %w", err)
	}

	get, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	if get.StatusCode != 200 {
		return fmt.Errorf("invalid status code %d: %s", get.StatusCode, func() string {
			b, _ := io.ReadAll(get.Body)
			return string(b)
		}())
	}

	buf := new(bytes.Buffer)
	teeReader := io.TeeReader(get.Body, buf)

	fj := filepath.Join(cacheDir, "final.tar.gz")
	create, err := os.Create(fj)
	if err != nil {
		return fmt.Errorf("os.Create(): %w", err)
	}
	_, err = io.Copy(create, teeReader)
	if err != nil {
		return fmt.Errorf("io.Copy(): %w", err)
	}

	return processData(cacheDir, buf)
}

func processData(cacheDir string, buf2 *bytes.Buffer) error {
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
			err := readAllRows[tables.BongLog](tarReader, func(t *tables.BongLog) {
				if !sBool(t.Won) {
					return
				}
				g := getGuildData(&guilds, t.GuildId)
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
			})
			if err != nil {
				return err
			}
		case "user-log.csv":
			err := readAllRows[tables.UserLog](tarReader, func(t *tables.UserLog) {
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
		v.TotalClicks = func() value.IntValSlice {
			a := value.IntValSlice{}
			for k2, v2 := range v.RawTotalClicks {
				a = append(a, value.IntVal{User: k2, Value: v2})
			}
			sort.Sort(a)
			return a[:IntMin(a.Len(), 10)]
		}()
		v.AvgClickSpeed = func() value.FloatValSlice {
			a := value.FloatValSlice{}
			for k2, v2 := range v.RawAvgSpeed {
				if len(v2) == 0 {
					continue
				}
				var z float64
				for _, i := range v2 {
					z += i
				}
				z = z / float64(len(v2))
				a = append(a, value.FloatVal{User: k2, Value: z})
			}
			sort.Sort(a)
			return a[:IntMin(a.Len(), 10)]
		}()
		v.SlowClickSpeed = func() value.FloatValSlice {
			a := value.FloatValSlice{}
			for k2, v2 := range v.RawSlowSpeed {
				if len(v2) == 0 {
					continue
				}
				z := v2[0]
				for _, i := range v2 {
					if i < z {
						z = i
					}
				}
				a = append(a, value.FloatVal{User: k2, Value: z})
			}
			sort.Sort(a)
			return a[:IntMin(a.Len(), 10)]
		}()
		v.FastClickSpeed = func() value.FloatValSlice {
			a := value.FloatValSlice{}
			for k2, v2 := range v.RawFastSpeed {
				if len(v2) == 0 {
					continue
				}
				z := v2[0]
				for _, i := range v2 {
					if i > z {
						z = i
					}
				}
				a = append(a, value.FloatVal{User: k2, Value: z})
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

func getGuildData(m *map[snowflake.ID]*CacheData, guildId snowflake.ID) *CacheData {
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

func readAllRows[T any](r io.Reader, f func(*T)) error {
	csvReader, err := gocsv.NewUnmarshaller(csv.NewReader(r), new(T))
	if err != nil {
		return fmt.Errorf("gocsv.NewUnmarshaller(): %w", err)
	}
	for {
		read, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("csvReader.Read(): %w", err)
		}
		if r, ok := read.(*T); ok {
			f(r)
		}
	}
	return nil
}

func sBool(won *bool) bool {
	if won == nil {
		return false
	}
	return *won
}

type yearItem struct {
	lock        *sync.RWMutex
	downloadUrl string
	cache       *CacheData
	dir         string
	guilds      map[string]*AsyncRead
}

func (y *yearItem) Compile() {
	y.lock.Lock()
	y.lock.Unlock()
}
