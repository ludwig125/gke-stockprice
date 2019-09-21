package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nlopes/slack"
)

func createAndSendMsgToSlack(token, channel, service, result, emoji string, start, finish time.Time) error {
	sl := NewSlack(token, channel)
	msg := createSlackMsg(service, start, finish, result)
	if err := sl.SendMessage(service, msg, emoji); err != nil {
		return fmt.Errorf("failed to SendMessage: %w", err)
	}
	return nil
}

// Slack is interface
type Slack interface {
	SendMessage(string, string, string) error
}

// SlackClient is struct to send slack API
type SlackClient struct {
	client  *slack.Client
	channel string
}

// NewSlack create new slack
func NewSlack(token, channel string) Slack {
	cl := slack.New(token)
	return SlackClient{cl, channel}
}

// SendMessage send message to slack API
func (sl SlackClient) SendMessage(service, msg, emoji string) error {
	// emoji
	// https://www.webfx.com/tools/emoji-cheat-sheet/
	channelID, timestamp, err := sl.client.PostMessage(sl.channel, slack.MsgOptionText(msg, false), slack.MsgOptionUsername(service+"-Bot"), slack.MsgOptionIconEmoji(emoji))
	if err != nil {
		return fmt.Errorf("failed to send message: %s", err)
	}
	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return nil
}

func createSlackMsg(service string, start, finish time.Time, msgContents string) string {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	processingTime := finish.Sub(start).Truncate(time.Second)

	msg := fmt.Sprintf("*%s が正常に終了しました。*\n", service)
	msg += fmt.Sprintf("起動時刻: %v\n", start.In(jst).Format("2006-01-02 15:04:05"))
	msg += fmt.Sprintf("終了時刻: %v\n", finish.In(jst).Format("2006-01-02 15:04:05"))
	msg += fmt.Sprintf("所要時間: %v\n\n", processingTime)
	msg += fmt.Sprintf("%s\n", msgContents)
	return msg
}
