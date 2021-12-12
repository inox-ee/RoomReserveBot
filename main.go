package main

import (
	"encoding/json"
	"fmt"
	"io"
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

func handleEventAPIEvent(api *slack.Client, eventApiEvent slackevents.EventsAPIEvent, body []byte, w http.ResponseWriter, db *badger.DB) {
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
				if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText("pong", false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "badger-update":
				if len(message) < 4 {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				err := db.Update(func(txn *badger.Txn) error {
					err := txn.Set([]byte(message[2]), []byte(message[3]))
					return err
				})
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				msg := fmt.Sprintf("set %s:%s", message[2], message[3])
				if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText(msg, false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "badger-view":
				var res string
				err := db.View(func(txn *badger.Txn) error {
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
				if _, _, err := api.PostMessage(event.Channel, slack.MsgOptionText(res, false)); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
	}
}

func main() {
	config, err := util.LoadConfig(".", "app")
	if err != nil {
		log.Fatal("cannot load config: ", err)
	}

	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Fatal("cannot open badgerDB: ", err)
	}
	defer db.Close()

	api := slack.New(config.SlackBotToken, slack.OptionDebug(true), slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)), slack.OptionAppLevelToken(config.SlackSocketToken))

	http.HandleFunc("/slack/test", func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, config.SlackSigningSecret)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bodyReader := io.TeeReader(r.Body, &verifier)
		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err = verifier.Ensure(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		eventsApiEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		handleEventAPIEvent(api, eventsApiEvent, body, w, db)
	})

	log.Println("[INFO] \x1b[33m⚡\x1b[0mServer listening")
	http.ListenAndServe(":8080", nil)
}
