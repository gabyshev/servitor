package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"

	"github.com/gabyshev/servitor/types"

	api "gopkg.in/telegram-bot-api.v4"
)

var (
	Conf       types.Config
	AuthConfig *oauth2.Config
)

func initConfig() {
	viper.SetEnvPrefix("servitor")
	viper.AutomaticEnv()
	Conf = types.Config{
		Entity:       viper.GetString("entity"),
		Token:        viper.GetString("token"),
		Host:         viper.GetString("host"),
		Project:      viper.GetString("project"),
		ClientSecret: viper.GetString("client_secret"),
	}
}

func Init() {
	initConfig()
	AuthConfig = getOauth2Config()
}

//get list of events
func getListOfEvents(c *types.Chat) {
	ctx := context.Background()
	client := AuthConfig.Client(ctx, c.Token)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve calendar Client %v", err)
	}

	uuid := uuid.New()
	chn := calendar.Channel{
		Id:      uuid.String(),
		Type:    "web_hook",
		Address: Conf.Host + "notification",
	}

	events := srv.Events.
		Watch("primary", &chn)

	channel, err := events.Do()

	if err != nil {
		log.Panic(err)
	}

	log.Printf("%+v\n", channel)
}

func getChat(c *datastore.Client, u api.Update) *types.Chat {
	ctx := context.Background()
	query := datastore.NewQuery(Conf.Entity).Filter("ID =", u.Message.Chat.ID)
	it := c.Run(ctx, query)
	var chat types.Chat
	_, _ = it.Next(&chat)
	if chat.ID == 0 {
		return nil
	}
	return &chat
}

func initBot(debug bool) *api.BotAPI {
	bot, err := api.NewBotAPI(Conf.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = debug

	log.Printf("Authorized on acc %s", bot.Self.UserName)
	_, err = bot.SetWebhook(api.NewWebhook(Conf.Host + bot.Token))

	if err != nil {
		log.Fatal(err)
	}
	return bot
}

func processUpdate(c *datastore.Client, u api.Update, bot *api.BotAPI) {
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
				getListOfEvents(chat)
				msg.Text = "you're already authorized"
			} else {
				ctx := context.Background()
				key, err := c.Put(ctx, datastore.IncompleteKey(Conf.Entity, nil), chat)
				if err != nil {
					log.Panic(err)
				}
				stateToken := strconv.FormatInt(key.ID, 10)
				authURL := AuthConfig.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)
				msg.Text = authURL
			}
		default:
			msg.Text = "Sorry, unknown command"
		}
		bot.Send(msg)
	}
}

func getOauth2Config() *oauth2.Config {
	b, err := ioutil.ReadFile(Conf.ClientSecret)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to create config form client secret file: %v", err)
	}
	return config
}

func main() {
	Init()
	ctx := context.Background()
	bot := initBot(true)
	client, err := datastore.NewClient(ctx, Conf.Project)
	if err != nil {
		log.Panic(err)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)

	//http.Handle("/", http.FileServer(http.Dir("./static")))
	http.Handle("/auth/google", &types.AuthHandler{Bot: bot, Client: client, Conf: &Conf, AuthConfig: AuthConfig})
	http.Handle("/notification", &types.NotificationHandler{Bot: bot, Client: client})
	go http.ListenAndServe(":5000", nil)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		processUpdate(client, update, bot)
	}
}
