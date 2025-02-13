package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/server"
	"nyiyui.ca/jts/tokens"
)

func getenvNonEmpty(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is not set", key)
	}
	return value
}

func main() {
	var dbPath string
	var bindAddress string
	var tokensPath string
	// secrets and their related options are in environment variables, others are in flags
	flag.StringVar(&bindAddress, "bind", "127.0.0.1:8080", "bind address")
	flag.StringVar(&dbPath, "db-path", "", "path to database. if empty, a default path like ~/.config/jts/jts.db is used")
	flag.StringVar(&tokensPath, "tokens-path", "", "path to tokens")
	flag.Parse()

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		log.Fatal(err)
	}
	tokenMap := map[string]server.TokenInfo{}
	err = json.Unmarshal(data, &tokenMap)
	if err != nil {
		log.Fatal(err)
	}
	tokenMap2 := map[tokens.TokenHash]server.TokenInfo{}
	for k, v := range tokenMap {
		tokenMap2[tokens.MustParseTokenHash(k)] = v
	}

	log.Printf("opening database...")
	db, err := database.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("new db: %s", err)
	}
	log.Printf("migrating database...")
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate db: %s", err)
	}
	log.Printf("database migrated.")

	authKey, err := hex.DecodeString(getenvNonEmpty("JTS_SERVER_STORE_AUTH_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	store := sessions.NewFilesystemStore("", authKey)

	s, err := server.New(&oauth2.Config{
		ClientID:     getenvNonEmpty("JTS_SERVER_OAUTH_CLIENT_ID"),
		ClientSecret: getenvNonEmpty("JTS_SERVER_OAUTH_CLIENT_SECRET"),
		Scopes:       []string{},
		Endpoint:     github.Endpoint,
		RedirectURL:  getenvNonEmpty("JTS_SERVER_OAUTH_REDIRECT_URI"),
	}, db, tokenMap2, store)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s...", bindAddress)
	log.Fatal(http.ListenAndServe(bindAddress, s))
}
