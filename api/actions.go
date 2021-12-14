package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/slack-go/slack"
)

const (
	roomNameAction  = "room-name"
	startTimeAction = "start-time"
	endTimeAction   = "end-time"
)

const (
	_selectRoomBlock = "_select-room"
	selectRoomBlock  = "select-room"
	confirmRoomBlock = "confirm-room"
)

func (srv *Server) handleActionPayload(payload *slack.InteractionCallback, w http.ResponseWriter) {
	switch payload.Type {
	case slack.InteractionTypeBlockActions:
		if len(payload.ActionCallback.BlockActions) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		action := payload.ActionCallback.BlockActions[0]
		switch action.BlockID {
		case selectRoomBlock:
			room := payload.BlockActionState.Values[_selectRoomBlock][roomNameAction].SelectedOption.Value
			startTime := payload.BlockActionState.Values[_selectRoomBlock][startTimeAction].SelectedTime
			endTime := payload.BlockActionState.Values[_selectRoomBlock][endTimeAction].SelectedTime

			text := slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("`%s` を `%s` ~ `%s` で予約しますか?", room, startTime, endTime), false, false)
			textSection := slack.NewSectionBlock(text, nil, nil)

			confirmButtonText := slack.NewTextBlockObject(slack.PlainTextType, "はい", false, false)
			confirmButton := slack.NewButtonBlockElement("", fmt.Sprintf("r_%s_%s_%s", room, startTime, endTime), confirmButtonText)
			confirmButton.WithStyle(slack.StylePrimary)

			denyButtonText := slack.NewTextBlockObject(slack.PlainTextType, "いいえ", false, false)
			denyButton := slack.NewButtonBlockElement("", "deny", denyButtonText)
			denyButton.WithStyle(slack.StyleDanger)

			actionBlock := slack.NewActionBlock(confirmRoomBlock, confirmButton, denyButton)

			fallbackText := slack.MsgOptionText("This client is not supported.", false)
			blocks := slack.MsgOptionBlocks(textSection, actionBlock)

			replaceOriginal := slack.MsgOptionReplaceOriginal(payload.ResponseURL)
			_, _, _, err := srv.slack.SendMessage("", replaceOriginal, fallbackText, blocks)
			handleError(err, w, http.StatusInternalServerError)
		case confirmRoomBlock:
			fmt.Println("here")
			if strings.HasPrefix(action.Value, "r") {
				reserveInfo := strings.Split(action.Value, "_")
				reserve := Reserve{
					Room:      reserveInfo[1],
					StartTime: reserveInfo[2],
					EndTime:   reserveInfo[3],
					User:      payload.User.Name,
				}
				go func() {
					startMsg := slack.MsgOptionText(fmt.Sprintf("<%s> `%s` の予約を開始します…", payload.User.Name, reserve.Room), false)
					_, _, err := srv.slack.PostMessage(payload.Channel.ID, startMsg)
					handleError(err, w, http.StatusInternalServerError)

					reserved_user, err := srv.ReserveRoom(&reserve)
					if reserved_user != "" {
						fallbackText := slack.MsgOptionText(fmt.Sprintf("既に予約されています by %s.", reserved_user), false)
						_, _, err := srv.slack.PostMessage(payload.Channel.ID, fallbackText)
						handleError(err, w, http.StatusInternalServerError)
						return
					}
					handleError(err, w, http.StatusInternalServerError)

					endMsg := slack.MsgOptionText(fmt.Sprintf("<%s> `%s` の予約が完了しました ( `%s` ~ `%s` )!", reserve.User, reserve.Room, reserve.StartTime, reserve.EndTime), false)
					_, _, err = srv.slack.PostMessage(payload.Channel.ID, endMsg)
					handleError(err, w, http.StatusInternalServerError)
				}()
			}

			deleteOriginal := slack.MsgOptionDeleteOriginal(payload.ResponseURL)
			_, _, _, err := srv.slack.SendMessage("", deleteOriginal)
			handleError(err, w, http.StatusInternalServerError)
		}
	}
}
