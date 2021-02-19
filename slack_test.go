package main

import (
	"testing"
	"time"
)

type SlackClientMock struct{}

func (s SlackClientMock) SendMessage(service, msg, emoji string) error {
	return nil
}

func NewTestSlack() Slack {
	return SlackClientMock{}
}

func TestCreateSlackMsg(t *testing.T) {
	service := "slack-test"
	start := time.Date(2019, 2, 1, 0, 0, 0, 0, time.Local)
	finish := time.Date(2019, 2, 1, 1, 23, 45, 67, time.Local)
	cont := `これはテストメッセージ\nslackのテスト\nです`

	want := `*slack-test が終了しました。*
起動時刻: 2019-02-01 00:00:00
終了時刻: 2019-02-01 01:23:45
所要時間: 1h23m45s

これはテストメッセージ\nslackのテスト\nです
`
	msg := createSlackMsg(service, start, finish, cont)
	t.Log(msg)
	if msg != want {
		t.Errorf("got msg:\n %#v\n want msg:\n %#v", msg, want)
	}
}
