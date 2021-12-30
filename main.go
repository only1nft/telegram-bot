package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const errMsg = "Something went wrong. Please try again later."
const (
	updateIdKey   = "main:update-id"
	inviteLinkKey = "main:invite-id"
)

var (
	debug bool
	token string

	bot         *tgbotapi.BotAPI
	repo        Repository
	conn        *rpc.Client
	collections map[string]*Collection
)

type CreateChatInviteLink struct {
	InviteLink string `json:"invite_link"`
}

type Collection struct {
	ID         string
	Name       string   `json:"name"`
	Mints      []string `json:"mints"`
	ChatID     int64    `json:"chatId"`
	InviteLink string   `json:"inviteLink"`
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.BoolVar(&debug, "debug", false, "debug")
	flag.StringVar(&token, "token", "", "Telegram token")
	flag.Parse()
}

func main() {
	var err error

	// Initialize repo
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger/only1/telegram"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	repo = Repository{Db: db}

	// Load collections
	collectionsJson, err := os.ReadFile("collections.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(collectionsJson, &collections); err != nil {
		log.Fatal(err)
	}
	for id, col := range collections {
		col.ID = id
	}

	// Initialize Solana rpc connection
	conn = rpc.New("https://only1.genesysgo.net/")

	// Start bot
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = debug
	log.Printf("started bot @%s", bot.Self.UserName)
	go watchdog(time.Hour * 6)

	updateId := 0
	updateIdStr, err := repo.GetString(updateIdKey)
	if err == nil {
		updateId, err = strconv.Atoi(updateIdStr)
		if err != nil {
			updateId = 0
		}
	}

	u := tgbotapi.NewUpdate(updateId + 1)
	u.Timeout = 30

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		processUpdate(update)
	}
}
