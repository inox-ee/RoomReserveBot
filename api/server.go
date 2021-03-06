package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"ventus-inc/Ventus_Office_ReserveBot/util"

	"github.com/dgraph-io/badger/v3"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Room struct {
	Name        string
	Description string
}

var rooms = []Room{
	{
		Name:        "Room1",
		Description: "5F 手前",
	},
	{
		Name:        "Room2",
		Description: "5F 奥",
	},
	{
		Name:        "Room3",
		Description: "6F",
	},
}

type Server struct {
	config util.Config
	store  *badger.DB
	slack  *slack.Client
}

func NewServer(config util.Config, store *badger.DB) *Server {
	slackApi := slack.New(config.SlackBotToken, slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)))

	return &Server{
		config: config,
		store:  store,
		slack:  slackApi,
	}
}

func handleError(err error, w http.ResponseWriter, status int) {
	if err != nil {
		log.Println(err)
		w.WriteHeader(status)
		return
	}
}

func (srv *Server) Start(addr string) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte("alive")); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/slack/events", slackVerificationMiddleware(srv.config, func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		handleError(err, w, http.StatusInternalServerError)

		eventsApiEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		handleError(err, w, http.StatusInternalServerError)

		srv.handleEventAPIEvent(eventsApiEvent, body, w)

	}))

	http.HandleFunc("/slack/actions", slackVerificationMiddleware(srv.config, func(w http.ResponseWriter, r *http.Request) {
		var payload *slack.InteractionCallback
		fmt.Println(r.FormValue("payload"))
		err := json.Unmarshal([]byte(r.FormValue("payload")), &payload)
		handleError(err, w, http.StatusInternalServerError)

		srv.handleActionPayload(payload, w)
	}))

	http.HandleFunc("/slack/events-verify", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			var res *slackevents.ChallengeResponse
			if err := json.Unmarshal(body, &res); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := w.Write([]byte(res.Challenge)); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	})

	log.Println("[INFO] \x1b[33m⚡\x1b[0mServer listening:", addr)
	err := http.ListenAndServe(fmt.Sprintf(":%s", addr), nil)
	return err
}

func (srv *Server) ReserveRoom(reserve *Reserve) (string, error) {
	reserved_user, err := reserve.IsConflict(srv)
	if err != nil {
		return reserved_user, err
	}
	err = srv.store.Update(func(txn *badger.Txn) error {
		key, val := reserve.FormatKV()
		err := txn.Set([]byte(key), []byte(val))
		return err
	})
	return "", err
}
