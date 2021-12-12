package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/inox-ee/TestSlack/util"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func handleEventAPIEvent(api *slack.Client, eventApiEvent slackevents.EventsAPIEvent, body []byte, w http.ResponseWriter) {
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
			}
		}
	}
}

func main() {
	config, err := util.LoadConfig(".", "app")
	if err != nil {
		log.Fatal("cannot load config: ", err)
	}

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
		handleEventAPIEvent(api, eventsApiEvent, body, w)
	})

	log.Println("[INFO] \x1b[33m⚡\x1b[0mServer listening")
	http.ListenAndServe(":8080", nil)
}
