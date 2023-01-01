package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gorilla/mux"
	"github.com/mrmelon54/bigben.mrmelon54.com/utils"
	"gopkg.in/yaml.v3"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	clientIdParse, err := snowflake.Parse(config.ClientId)
	if err != nil {
		log.Fatal("Parse ClientId snowflake: ", err)
	}

	oauthClient := oauth2.New(clientIdParse, config.ClientSecret)

	log.Println("Building data cache")
	yearMap := make(map[string]*yearItem)
	for k, v := range config.Year {
		yp := filepath.Join(config.Cache, k)
		yearMap[k] = &yearItem{
			lock:        &sync.RWMutex{},
			downloadUrl: v,
			dir:         yp,
			guilds:      make(map[string]*utils.AsyncRead),
		}
		if _, err := os.Stat(yp); errors.Is(err, os.ErrNotExist) {
			log.Printf("Cache missing for %s, downloading...\n", k)
			err = utils.DownloadData(yp, v)
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
	router.HandleFunc("/login", func(rw http.ResponseWriter, req *http.Request) {
		oauthClient.GenerateAuthorizationURL(config.RedirectUrl, discord.PermissionsNone, 0, true, discord.OAuth2ScopeIdentify, discord.OAuth2ScopeGuilds)
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
					y.guilds[guildId] = utils.NewAsyncRead(y.dir)
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

type yearItem struct {
	lock        *sync.RWMutex
	downloadUrl string
	cache       *utils.CacheData
	dir         string
	guilds      map[string]*utils.AsyncRead
}
