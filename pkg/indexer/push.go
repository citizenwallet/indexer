package indexer

import (
	"fmt"
)

type PushToken struct {
	Token   string
	Account string
}

type PushMessage struct {
	Tokens []*PushToken
	Title  string
	Body   string
}

const PushMessageAnonymousDescriptionTitle = "%s %s (%s) received"
const PushMessageAnonymousDescriptionBody = "%s"
const PushMessageAnonymousTitle = "%s"
const PushMessageAnonymousBody = "%s %s received"
const PushMessageTitle = "%s - %s"
const PushMessageBody = "%s %s received from %s"

func NewAnonymousPushMessage(token []*PushToken, community, amount, symbol string, txdata *TransferData) *PushMessage {
	if txdata != nil {
		return &PushMessage{
			Tokens: token,
			Title:  fmt.Sprintf(PushMessageAnonymousDescriptionTitle, amount, community, symbol),
			Body:   fmt.Sprintf(PushMessageAnonymousDescriptionBody, txdata.Description),
		}
	}

	return &PushMessage{
		Tokens: token,
		Title:  fmt.Sprintf(PushMessageAnonymousTitle, community),
		Body:   fmt.Sprintf(PushMessageAnonymousBody, amount, symbol),
	}
}

func NewPushMessage(token []*PushToken, community, name, amount, symbol, username string) *PushMessage {
	return &PushMessage{
		Tokens: token,
		Title:  fmt.Sprintf(PushMessageTitle, community, name),
		Body:   fmt.Sprintf(PushMessageBody, amount, symbol, username),
	}
}
