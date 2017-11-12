package types

import (
	//"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	api "gopkg.in/telegram-bot-api.v4"
)

type Chat struct {
	ID           int64
	Username     string
	Firstname    string
	Lastname     string
	Token        *oauth2.Token
	IsAuthorized bool
	Created      time.Time
	Updated      time.Time
}

type Config struct {
	Entity       string
	Token        string
	Host         string
	Project      string
	ClientSecret string
}

type AuthHandler struct {
	Bot        *api.BotAPI
	Client     *datastore.Client
	Conf       *Config
	AuthConfig *oauth2.Config
}

type NotificationHandler struct {
	Bot    *api.BotAPI
	Client *datastore.Client
}

func (h *NotificationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	log.Printf("\nGot response: %+v\n", r)
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	id, err := strconv.ParseInt(state, 10, 64)
	if err != nil {
		log.Panic(err)
	}

	key := datastore.IDKey(h.Conf.Entity, id, nil)
	var chat Chat

	_, err = h.Client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		err = h.Client.Get(ctx, key, &chat)
		if err != nil {
			return err
		}

		token, err := h.AuthConfig.Exchange(oauth2.NoContext, code)
		if err != nil {
			return err
		}

		chat.Token = token
		chat.IsAuthorized = true
		chat.Updated = time.Now()

		_, err = tx.Put(key, &chat)
		return err
	})

	if err != nil {
		log.Panic(err)
	}

	msg := api.NewMessage(chat.ID, "you are successfully authorized")
	h.Bot.Send(msg)
}
