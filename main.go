package main

import (
	"bytes"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"github.com/1f349/mjwt"
	"github.com/discord-plays/bigben.discord-plays.xyz/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	//go:embed www/assets/style.css
	rawStyle []byte

	homePage = func() *template.Template {
		parse, err := template.New("home").Parse(rawHomePage)
		if err != nil {
			log.Fatal(err)
		}
		return parse
	}()
	leaderboardPage = func() *template.Template {
		parse, err := template.New("leaderboard").Funcs(template.FuncMap{
			"renderTime": func(a float64) string {
				return time.Duration(a * float64(time.Second)).Truncate(time.Millisecond).String()
			},
			"renderUser": func(users map[snowflake.ID]string, id snowflake.ID) string {
				u := users[id]
				if u == "" || len(u) <= 2 {
					return "Unknown User"
				}
				if u[len(u)-2:len(u)] == "#0" {
					return u[:len(u)-2]
				}
				if u == "" || len(u) <= 5 {
					return "Unknown User"
				}
				return u[:len(u)-5]
			},
		}).Parse(rawLeaderboardPage)
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

	wd := filepath.Dir(configPath)

	signer, err := mjwt.NewMJwtSignerFromFileOrCreate("bigben.discord-plays.xyz", filepath.Join(wd, "session-key.private.pem"), cryptoRand.Reader, 4096)
	if err != nil {
		log.Fatal("NewMJwtSignerFromFileOrCreate(): ", err)
	}

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
			Guilds:      make(map[snowflake.ID]*utils.AsyncRead),
		}
		if _, err := os.Stat(yp); errors.Is(err, os.ErrNotExist) {
			log.Printf("Cache missing for %s, downloading...\n", k)
			err = utils.DownloadData(yp, v)
			if err != nil {
				log.Fatal("Failed to download and process data: ", err)
			}
		}
		dir, err := os.ReadDir(yp)
		if err != nil {
			log.Fatalf("Failed to read '%s' directory", yp)
		}
		for _, i := range dir {
			v := i.Name()
			if strings.HasSuffix(v, ".json") {
				v2 := v[:len(v)-5]
				v3, err := snowflake.Parse(v2)
				if err != nil {
					log.Fatalf("Failed to parse snowflake '%s'", v2)
					return
				}
				yearMap[k].Guilds[v3] = utils.NewAsyncRead(filepath.Join(yp, v))
			}
		}
	}

	router := mux.NewRouter()
	router.HandleFunc("/", checkLogin(signer, oauthClient, func(rw http.ResponseWriter, req *http.Request, session *oauth2.Session, data loginData) {
		if session == nil || !data.LoggedIn {
			err := homePage.Execute(rw, struct{ Login loginData }{Login: data})
			if err != nil {
				log.Println("homePage.Execute(no login): ", err)
			}
			return
		}
		guilds, err := oauthClient.GetGuilds(*session)
		if err != nil {
			http.Error(rw, "500 Failed to get guild data", http.StatusInternalServerError)
			return
		}
		yearGuilds := make(map[string][]guildItem)
		for k, v := range yearMap {
			yearGuilds[k] = []guildItem{}
			v.lock.RLock()
			for _, i := range guilds {
				if _, ok := v.Guilds[i.ID]; ok {
					guildIcon := ""
					if i.Icon != nil {
						iconRaw := *i.Icon
						conf := discord.DefaultCDNConfig()
						if strings.HasPrefix(iconRaw, "a_") && !conf.Format.Animated() {
							conf.Format = discord.FileFormatGIF
						}
						conf.Values["size"] = 512
						guildIcon = discord.GuildIcon.URL(conf.Format, conf.Values, i.ID.String(), iconRaw)
					}
					discord.User{}.EffectiveAvatarURL()
					yearGuilds[k] = append(yearGuilds[k], guildItem{
						Id:   i.ID.String(),
						Name: i.Name,
						Icon: guildIcon,
					})
				}
			}
			v.lock.RUnlock()
		}
		err = homePage.Execute(rw, struct {
			Login  loginData
			Guilds map[string][]guildItem
		}{Login: data, Guilds: yearGuilds})
		if err != nil {
			log.Println("homePage.Execute(with login): ", err)
		}
	}))
	router.HandleFunc("/bigben.png", func(rw http.ResponseWriter, req *http.Request) {
		http.ServeContent(rw, req, "bigben.png", time.Now(), bytes.NewReader(rawLogo))
	})
	router.HandleFunc("/style.css", func(rw http.ResponseWriter, req *http.Request) {
		http.ServeContent(rw, req, "style.css", time.Now(), bytes.NewReader(rawStyle))
	})
	router.HandleFunc("/login", func(rw http.ResponseWriter, req *http.Request) {
		var (
			query = req.URL.Query()
			code  = query.Get("code")
			state = query.Get("state")
		)
		if code != "" && state != "" {
			oaSess, _, err := oauthClient.StartSession(code, state)
			if err != nil {
				http.Error(rw, "500 Error starting session", http.StatusInternalServerError)
				return
			}
			marshal, err := json.Marshal(oaSess)
			if err != nil {
				http.Error(rw, "500 Error saving session", http.StatusInternalServerError)
				return
			}
			encrypt, err := rsa.EncryptOAEP(sha256.New(), cryptoRand.Reader, signer.PublicKey(), marshal, []byte("bigben-session"))
			if err != nil {
				http.Error(rw, "500 Error encrypting session", http.StatusInternalServerError)
				return
			}
			encryptB64 := base64.RawURLEncoding.EncodeToString(encrypt)
			http.SetCookie(rw, &http.Cookie{Name: "bigben-session", Value: encryptB64, Path: "/", Expires: time.Now().Add(time.Hour * 24 * 7)})
			http.Redirect(rw, req, "/", http.StatusFound)
			return
		}
		http.Redirect(rw, req, oauthClient.GenerateAuthorizationURL(config.RedirectUrl, discord.PermissionsNone, 0, true, discord.OAuth2ScopeIdentify, discord.OAuth2ScopeGuilds), http.StatusFound)
	})
	router.HandleFunc("/logout", func(rw http.ResponseWriter, req *http.Request) {
		http.SetCookie(rw, &http.Cookie{Name: "bigben-session", Value: "", Path: "/", Expires: time.Now().Add(-time.Second)})
	})
	router.HandleFunc("/{year}/{guild}", checkLogin(signer, oauthClient, func(rw http.ResponseWriter, req *http.Request, session *oauth2.Session, data loginData) {
		if session == nil || !data.LoggedIn {
			http.Redirect(rw, req, "/", http.StatusFound)
			return
		}
		vars := mux.Vars(req)
		yearNum := vars["year"]
		guildId, err := snowflake.Parse(vars["guild"])
		if err != nil {
			http.Error(rw, "400 Invalid guild", http.StatusBadRequest)
			return
		}
		if y, ok := yearMap[yearNum]; ok {
			if y.lock.TryRLock() {
				y2 := y.Guilds[guildId]
				y.lock.RUnlock()
				if y2 == nil {
					y.lock.Lock()
					y.Guilds[guildId] = utils.NewAsyncRead(y.dir)
					y.lock.Unlock()
				}
				c := y2.Get()
				err = leaderboardPage.Execute(rw, struct {
					Login loginData
					HasC  bool
					Year  string
					C     *utils.CacheData
				}{
					Login: data,
					HasC:  c != nil,
					Year:  yearNum,
					C:     c,
				})
				if err != nil {
					log.Println("leaderboardPage.Execute(with login): ", err)
				}
				return
			}
			http.Error(rw, "404 Not Found", http.StatusNotFound)
			return
		}
		http.Error(rw, "404 Not Found", http.StatusNotFound)
	})).Methods(http.MethodGet)

	log.Printf("Serving HTTP on '%s'\n", config.Listen)
	srv := &http.Server{
		Addr:    config.Listen,
		Handler: router,
	}
	log.Fatal("ListenAndServe(): ", srv.ListenAndServe())
}

func checkLogin(signer mjwt.Signer, oauthClient oauth2.Client, f func(rw http.ResponseWriter, req *http.Request, session *oauth2.Session, data loginData)) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("bigben-session")
		if err == nil {
			decryptDecode, err := base64.RawURLEncoding.DecodeString(cookie.Value)
			if err != nil {
				http.Error(rw, "500 Failed to decode cookie value", http.StatusInternalServerError)
				return
			}
			decrypt, err := rsa.DecryptOAEP(sha256.New(), nil, signer.PrivateKey(), decryptDecode, []byte("bigben-session"))
			if err != nil {
				http.Error(rw, "500 Failed to read cookie value", http.StatusInternalServerError)
				return
			}
			var oaSess oauth2.Session
			if json.Unmarshal(decrypt, &oaSess) != nil {
				http.Error(rw, "500 Error loading session", http.StatusInternalServerError)
				return
			}
			user, err := oauthClient.GetUser(oaSess)
			if err != nil {
				http.Error(rw, "500 Failed to get user data", http.StatusInternalServerError)
				return
			}
			f(rw, req, &oaSess, loginData{
				LoggedIn: true,
				UserId:   user.ID.String(),
				UserTag:  user.Tag(),
				UserIcon: user.EffectiveAvatarURL(),
			})
			return
		}
		f(rw, req, nil, loginData{
			LoggedIn: false,
			UserId:   "",
			UserTag:  "",
			UserIcon: "",
		})
	}
}

type yearItem struct {
	lock        *sync.RWMutex
	downloadUrl string
	cache       *utils.CacheData
	dir         string
	Guilds      map[snowflake.ID]*utils.AsyncRead
}

type guildItem struct {
	Id   string
	Name string
	Icon string
}

type loginData struct {
	LoggedIn bool
	UserId   string
	UserTag  string
	UserIcon string
}
