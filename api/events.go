package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func (srv *Server) handleEventAPIEvent(eventApiEvent slackevents.EventsAPIEvent, body []byte, w http.ResponseWriter) {
	switch eventApiEvent.Type {

	case slackevents.URLVerification:
		var res *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &res)
		handleError(err, w, http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		_, err = w.Write([]byte(res.Challenge))
		handleError(err, w, http.StatusInternalServerError)

	case slackevents.CallbackEvent:
		innerEvent := eventApiEvent.InnerEvent

		switch event := innerEvent.Data.(type) {

		case *slackevents.AppMentionEvent:
			srv.handleAppMentionEvent(event, w)
		}
	}
}

func (srv *Server) handleAppMentionEvent(event *slackevents.AppMentionEvent, w http.ResponseWriter) {
	message := strings.Split(event.Text, " ")
	var command string

	if len(message) < 2 {
		message = append(message, "list")
	}
	command = message[1]

	switch command {
	case "ping":
		_, _, err := srv.slack.PostMessage(event.Channel, slack.MsgOptionText("pong", false))
		handleError(err, w, http.StatusInternalServerError)

	case "reserve":
		text := slack.NewTextBlockObject(slack.MarkdownType, "利用する部屋を選択してください", false, false)
		textSection := slack.NewSectionBlock(text, nil, nil)

		roomOpt := make([]*slack.OptionBlockObject, 0, len(rooms))
		for _, room := range rooms {
			optionText := slack.NewTextBlockObject(slack.PlainTextType, room.Name, false, false)
			descriptionText := slack.NewTextBlockObject(slack.PlainTextType, room.Description, false, false)
			roomOpt = append(roomOpt, slack.NewOptionBlockObject(room.Name, optionText, descriptionText))
		}
		roomPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "Select room", false, false)
		roomSelectMenu := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, roomPlaceholder, "room-name", roomOpt...)

		startPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "Start time", false, false)
		startTimePicker := slack.NewTimePickerBlockElement("start-time")
		startTimePicker.Placeholder = startPlaceholder

		endPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "End time", false, false)
		endTimePicker := slack.NewTimePickerBlockElement("end-time")
		endTimePicker.Placeholder = endPlaceholder

		confirmButtonText := slack.NewTextBlockObject(slack.PlainTextType, "決定", false, false)
		confirmButton := slack.NewButtonBlockElement("", "confirm", confirmButtonText)
		confirmButton.WithStyle(slack.StylePrimary)

		inputBlock := slack.NewActionBlock(_selectRoomBlock, roomSelectMenu, startTimePicker, endTimePicker)
		actionBlock := slack.NewActionBlock(selectRoomBlock, confirmButton)

		fallbackText := slack.MsgOptionText("This client is not supported.", false)
		blocks := slack.MsgOptionBlocks(textSection, inputBlock, actionBlock)

		_, err := srv.slack.PostEphemeral(event.Channel, event.User, fallbackText, blocks)
		handleError(err, w, http.StatusInternalServerError)

	case "list":
		res := []Reserve{}
		err := srv.store.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			it := txn.NewIterator(opts)
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				k := item.Key()
				err := item.Value(func(v []byte) error {
					newRes := ParseReserveKV(string(k), string(v))
					res = append(res, newRes)
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		handleError(err, w, http.StatusInternalServerError)
		var msg string
		if len(res) == 0 {
			msg = fmt.Sprintf("%s の予約状況\n\n%s", time.Now().Format("2006-01-02"), "DB が空のようです")
		} else {
			msg = fmt.Sprintf("%s の予約状況\n\n%s", time.Now().Format("2006-01-02"), FormatViewByRoom(res))
		}
		_, _, err = srv.slack.PostMessage(event.Channel, slack.MsgOptionText(msg, false))
		handleError(err, w, http.StatusInternalServerError)

	case "raw":
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
					res += string(k) + "_" + string(v) + "\n"
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		handleError(err, w, http.StatusInternalServerError)
		_, _, err = srv.slack.PostMessage(event.Channel, slack.MsgOptionText(res, false))
		handleError(err, w, http.StatusInternalServerError)

	case "reset":
		err := srv.store.DropAll()
		handleError(err, w, http.StatusInternalServerError)
		_, _, err = srv.slack.PostMessage(event.Channel, slack.MsgOptionText("会議室の予約をリセットしました", false))
		handleError(err, w, http.StatusInternalServerError)
	default:
		fallbackText := slack.MsgOptionText("そんなコマンド知らないよ", false)
		_, err := srv.slack.PostEphemeral(event.Channel, event.User, fallbackText)
		handleError(err, w, http.StatusInternalServerError)
	}
}
