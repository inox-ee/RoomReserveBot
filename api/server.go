package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"github.com/inox-ee/TestSlack/util"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Server struct {
	config util.Config
	store  *badger.DB
	slack  *slack.Client
}

func NewServer(config util.Config, store *badger.DB) *Server {
	slackApi := slack.New(config.SlackBotToken, slack.OptionDebug(true), slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)), slack.OptionAppLevelToken(config.SlackSocketToken))

	return &Server{
		config: config,
		store:  store,
		slack:  slackApi,
	}
}

func handleError(err error, w http.ResponseWriter, status int) {
	if err != nil {
		w.WriteHeader(status)
		return
	}
}

func (srv *Server) Start(addr string) error {
	http.HandleFunc("/slack/events", slackVerificationMiddleware(srv.config, func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		handleError(err, w, http.StatusInternalServerError)

		eventsApiEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		handleError(err, w, http.StatusInternalServerError)

		srv.handleEventAPIEvent(eventsApiEvent, body, w)

	}))

	log.Println("[INFO] \x1b[33m⚡\x1b[0mServer listening")
	err := http.ListenAndServe(fmt.Sprintf(":%s", addr), nil)
	return err
}

func (srv *Server) handleEventAPIEvent(eventApiEvent slackevents.EventsAPIEvent, body []byte, w http.ResponseWriter) {
	switch eventApiEvent.Type {
	case slackevents.URLVerification:
		var res *slackevents.ChallengeResponse
		if err := json.Unmarshal(body, &res); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(res.Challenge)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case slackevents.CallbackEvent:
		innerEvent := eventApiEvent.InnerEvent
		switch event := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			message := strings.Split(event.Text, " ")
			if len(message) < 2 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			command := message[1]
			switch command {
			case "ping":
				if _, _, err := srv.slack.PostMessage(event.Channel, slack.MsgOptionText("pong", false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "badger-update":
				if len(message) < 4 {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				err := srv.store.Update(func(txn *badger.Txn) error {
					err := txn.Set([]byte(message[2]), []byte(message[3]))
					return err
				})
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				msg := fmt.Sprintf("set %s:%s", message[2], message[3])
				if _, _, err := srv.slack.PostMessage(event.Channel, slack.MsgOptionText(msg, false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "badger-view":
				var res string
				err := srv.store.View(func(txn *badger.Txn) error {
					opts := badger.DefaultIteratorOptions
					opts.PrefetchSize = 10
					it := txn.NewIterator(opts)
					defer it.Close()
					for it.Rewind(); it.Valid(); it.Next() {
						item := it.Item()
						k := item.Key()
						err := item.Value(func(v []byte) error {
							res = fmt.Sprintf("key=%s, value=%s\n", k, v)
							return nil
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if res == "" {
					res = "DB が空のようです"
				}
				if _, _, err := srv.slack.PostMessage(event.Channel, slack.MsgOptionText(res, false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
	}
}
