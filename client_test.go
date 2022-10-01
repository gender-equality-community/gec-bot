package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type dummyClient struct {
	err      bool
	read     bool
	msg      string
	presence bool
}

func (dummyClient) AddEventHandler(handler whatsmeow.EventHandler) uint32 { return 1 }
func (dummyClient) GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	out := make(chan whatsmeow.QRChannelItem, 1)

	out <- whatsmeow.QRChannelItem{}

	return out, nil
}

func (c dummyClient) Connect() error {
	if c.err {
		return fmt.Errorf("an error")
	}

	return nil
}

func (dummyClient) Disconnect() {}

func (c *dummyClient) MarkRead([]string, time.Time, types.JID, types.JID) error {
	if c.err {
		return fmt.Errorf("an error")
	}

	c.read = true

	return nil
}

func (c *dummyClient) SendMessage(_ context.Context, _ types.JID, _ string, msg *waProto.Message) (r whatsmeow.SendResponse, err error) {
	if c.err {
		err = fmt.Errorf("an error")
	}

	if c.msg == "" {
		c.msg = msg.GetConversation()
	}

	return
}

func (c *dummyClient) SendPresence(types.Presence) error {
	c.presence = true

	return nil
}

func TestNew(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Error("expected error")
		}
	}()

	_, err := New(nil, Redis{})
	if err != nil {
		t.Error("expected error")
	}
}

func TestClient_Handle(t *testing.T) {
	oldBoottime := boottime
	defer func() {
		boottime = oldBoottime
	}()
	boottime = time.Time{}

	for _, test := range []struct {
		name           string
		wc             *dummyClient
		rc             *dummyRedis
		msg            *events.Message
		expectRead     bool
		expectResponse string
	}{
		{"Messages from me should be ignored", new(dummyClient), new(dummyRedis), &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: true, Sender: dummyJid()}, ID: "123"}}, false, ""},
		{"Redis errors should bomb out, but not mark read", new(dummyClient), &dummyRedis{err: true}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}}, false, ""},
		{"Greetings should be recognised as such", new(dummyClient), new(dummyRedis), &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{Conversation: stringRef("Hello!")}}, true, greetingResponse},
		{"Where we've already messaged someone, don't message again", new(dummyClient), &dummyRedis{idExists: true}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{Conversation: stringRef("I would like to talk to somebody please")}}, true, ""},
		{"Where we've not messaged someone recently, message again", new(dummyClient), &dummyRedis{idExists: false}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{Conversation: stringRef("I would like to talk to somebody please")}}, true, thankyouResponse},
		{"Brand new recipients should get a new ID", new(dummyClient), &dummyRedis{noJID: true}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{Conversation: stringRef("Hello!")}}, true, greetingResponse},
		{"Messages using new format should work fine", new(dummyClient), &dummyRedis{noJID: true}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: stringRef("Hi!")}}}, true, greetingResponse},
		{"Messages where no message is found should be dropped", new(dummyClient), &dummyRedis{noJID: true}, &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: false, Sender: dummyJid()}, ID: "123"}, Message: &waProto.Message{}}, false, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := Client{
				c: test.wc,
				r: Redis{
					client: test.rc,
				},
			}

			c.handler(test.msg)

			t.Run("message acked correctly", func(t *testing.T) {
				if test.expectRead != test.wc.read {
					t.Errorf("expected %v, received %v", test.expectRead, test.wc.read)
				}
			})

			t.Run("presence is set", func(t *testing.T) {
				if !test.wc.presence {
					t.Error("presence was not set")
				}
			})

			t.Run("correct message is sent", func(t *testing.T) {
				if test.expectResponse != test.wc.msg {
					t.Errorf("expected %q, received %q", test.expectResponse, test.wc.msg)
				}
			})
		})
	}
}

func TestClient_Handle_SkipOldMessage(t *testing.T) {
	c := Client{
		c: &dummyClient{},
		r: Redis{
			client: &dummyRedis{},
		},
	}

	c.handler(&events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				IsFromMe: false,
				Sender:   dummyJid(),
			},
			ID:        "123",
			Timestamp: time.Unix(578430600, 0),
		},
		Message: &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: stringRef("Hi!"),
			},
		},
	})

	if c.c.(*dummyClient).msg != "" {
		t.Errorf("unexpected response %q", c.c.(*dummyClient).msg)
	}
}

func TestClient_HandleResponse(t *testing.T) {
	cleanMsg := Message{ID: "some-user", Message: "Hello, World!"}

	for _, test := range []struct {
		name      string
		wc        *dummyClient
		rc        *dummyRedis
		msg       Message
		expectErr bool
	}{
		{"Message is malformed and has no ID should error", new(dummyClient), new(dummyRedis), Message{}, true},
		{"Redis errors float error up", new(dummyClient), &dummyRedis{err: true}, cleanMsg, true},
		{"Whatsapp messages float up", &dummyClient{err: true}, new(dummyRedis), cleanMsg, true},
		{"Unknown recipient should fail", &dummyClient{err: true}, &dummyRedis{noJID: true}, Message{ID: "foo"}, true},
		{"Valid payloads and recipients should succeed", new(dummyClient), new(dummyRedis), cleanMsg, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := Client{
				c: test.wc,
				r: Redis{
					client: test.rc,
				},
			}

			err := c.HandleResponse(test.msg)
			if err == nil && test.expectErr {
				t.Errorf("expected error")
			} else if err != nil && !test.expectErr {
				t.Errorf("unexpected error: %+v", err)
			}

			t.Run("correct message is sent", func(t *testing.T) {
				if !test.expectErr && test.msg.Message != test.wc.msg {
					t.Errorf("expected %q, received %q", test.msg.Message, test.wc.msg)
				}
			})
		})
	}
}

func dummyJid() types.JID {
	return types.JID{
		User:   "some-user-123",
		Agent:  1,
		Device: 1,
		Server: "s.whatsapp.net",
		AD:     true,
	}
}
