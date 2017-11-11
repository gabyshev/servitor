package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/gabyshev/servitor/auth"
	"github.com/gabyshev/servitor/types"

	api "gopkg.in/telegram-bot-api.v4"
)

const (
	EnvPrefix = "servitor"
)

type AuthHandler struct {
	bot    *api.BotAPI
	client *datastore.Client
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	entity := viper.GetString("entity")
	//code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	id, err := strconv.ParseInt(state, 10, 64)
	if err != nil {
		log.Panic(err)
	}

	key := datastore.IDKey(entity, id, nil)
	log.Printf("search key is: %q", key)
	var chat types.Chat

	err = h.client.Get(ctx, key, &chat)
	if err != nil {
		log.Panic(err)
	}

	chat.IsAuthorized = true
	chat.Updated = time.Now()

	//TODO: exchange code for token ans save it
	msg := api.NewMessage(chat.ID, "you are successfully authorized")
	h.bot.Send(msg)
}

func getChat(c *datastore.Client, u api.Update) *types.Chat {
	entity := viper.GetString("entity")
	ctx := context.Background()
	query := datastore.NewQuery(entity).Filter("ID =", u.Message.Chat.ID)
	it := c.Run(ctx, query)
	var chat types.Chat
	_, _ = it.Next(&chat)
	if chat.ID == 0 {
		return nil
	}
	return &chat
}

func initBot(debug bool) *api.BotAPI {
	token := viper.GetString("token")
	host := viper.GetString("host")
	bot, err := api.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = debug

	log.Printf("Authorized on acc %s", bot.Self.UserName)
	_, err = bot.SetWebhook(api.NewWebhook(host + bot.Token))

	if err != nil {
		log.Fatal(err)
	}
	return bot
}

func processUpdate(c *datastore.Client, u api.Update, bot *api.BotAPI) {
	entity := viper.GetString("entity")
	chat := getChat(c, u)
	if chat == nil {
		chat = &types.Chat{
			ID:        u.Message.Chat.ID,
			Username:  u.Message.From.UserName,
			Firstname: u.Message.From.FirstName,
			Lastname:  u.Message.From.LastName,
			Created:   time.Now(),
		}
	}

	if u.Message.IsCommand() {
		msg := api.NewMessage(chat.ID, "")

		switch u.Message.Command() {
		case "start":
			if chat.IsAuthorized {
				//smth else
				log.Println("chat is already authorized")
			}
			log.Printf("chat is new")
			ctx := context.Background()
			key, err := c.Put(ctx, datastore.IncompleteKey(entity, nil), chat)
			if err != nil {
				log.Panic(err)
			}
			log.Printf("key is : %q", key)
			stateToken := strconv.FormatInt(key.ID, 10)
			log.Printf("state is : %q", stateToken)
			authURL := auth.GetAuthUrl(stateToken)
			msg.Text = authURL
		default:
			msg.Text = "Sorry, unknown command"
		}
		bot.Send(msg)
	}
}

func main() {
	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	project := viper.GetString("project")

	ctx := context.Background()
	bot := initBot(true)
	client, err := datastore.NewClient(ctx, project)
	if err != nil {
		log.Panic(err)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)

	//http.Handle("/", http.FileServer(http.Dir("./static")))
	http.Handle("/auth/google", &AuthHandler{bot: bot, client: client})
	go http.ListenAndServe(":5000", nil)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		processUpdate(client, update, bot)
	}
}
