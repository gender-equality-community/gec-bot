package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type whatsappClient interface {
	AddEventHandler(handler whatsmeow.EventHandler) uint32
	GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	Connect() error
	Disconnect()
	MarkRead([]string, time.Time, types.JID, types.JID) error
	SendMessage(context.Context, types.JID, string, *waProto.Message) (whatsmeow.SendResponse, error)
}

type Client struct {
	c whatsappClient
	r Redis
}

func New(db *sqlstore.Container, r Redis) (c Client, err error) {
	c.r = r

	deviceStore, err := db.GetFirstDevice()
	if err != nil {
		return
	}

	clientLog := waLog.Stdout("Client", LogLevel, true)

	c.c = whatsmeow.NewClient(deviceStore, clientLog)
	c.c.AddEventHandler(c.handler)

	return c, c.connect()
}

func (c *Client) connect() (err error) {
	wc, ok := c.c.(*whatsmeow.Client)

	if ok && wc.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := c.c.GetQRChannel(context.Background())

		err = c.c.Connect()
		if err != nil {
			return
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		}

		return
	}

	return c.c.Connect()
}

func (c *Client) disconnect() {
	c.c.Disconnect()
}

func (c Client) handler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	}
}

func (c Client) handleMessage(msg *events.Message) {
	if msg.Info.IsFromMe {
		return
	}

	ctx := context.Background()
	jid := msg.Info.Sender.ToNonAD()

	// lookup id for jid
	var (
		id  string
		err error
	)

	id, err = c.r.JIDToID(jid)
	if err != nil {
		id, err = c.r.MintID(jid)
		if err != nil {
			return
		}
	}

	txt := msg.Message.GetConversation()
	err = c.r.Produce(id, txt)
	if err != nil {
		return
	}

	c.c.MarkRead([]types.MessageID{msg.Info.ID}, time.Now(), jid, jid)

	if isMaybeGreeting(txt) {
		_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef(greetingResponse),
		})
		if err != nil {
			log.Print(err)
		}

		c.disclaimer(jid, id)

		return
	}

	// If we haven't messaged this person the standard 'thanks for your response' in the last
	// 30 minutes then do so now
	if !c.r.HasRecentlySent(thankyouKey(id)) {
		c.r.MarkRecentlySent(thankyouKey(id), time.Minute*30)
		_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef(thankyouResponse),
		})

		if err != nil {
			log.Print(err)
		}
	}

	c.disclaimer(jid, id)
}

func (c Client) disclaimer(jid types.JID, id string) {
	// If we haven't sent the disclaimer in 24 hours, then do that
	if !c.r.HasRecentlySent(disclaimerKey(id)) {
		ctx := context.Background()

		c.r.MarkRecentlySent(disclaimerKey(id), time.Hour*24)
		_, err := c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef(disclaimerResponse),
		})

		if err != nil {
			log.Print(err)
		}
	}

}

func (c Client) ResponseQueue(m chan Message) {
	for msg := range m {
		err := c.HandleResponse(msg)
		if err != nil {
			log.Printf("%#v", err)
		}
	}
}

func (c Client) HandleResponse(msg Message) (err error) {
	ctx := context.Background()

	if msg.ID == "" {
		return fmt.Errorf("malformed Message")
	}

	jid, err := c.r.IDToJID(msg.ID)
	if err != nil || jid.IsEmpty() {
		return
	}

	_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
		Conversation: stringRef(msg.Message),
	})

	return
}

func stringRef(s string) *string {
	return &s
}

func disclaimerId(s string) string {
	return fmt.Sprintf("disclaimer:%s", s)
}
