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

	var txt string
	if msg.Message.ExtendedTextMessage != nil {
		txt = msg.Message.ExtendedTextMessage.GetText()
	} else {
		txt = msg.Message.GetConversation()
	}

	if txt == "" {
		return
	}

	err = c.r.Produce(id, txt)
	if err != nil {
		return
	}

	err = c.c.MarkRead([]types.MessageID{msg.Info.ID}, time.Now(), jid, jid)
	if err != nil {
		return
	}

	if isMaybeGreeting(txt) {
		err = c.sendAutoResponse(jid, id, greetingResponse)
	} else if !c.r.HasRecentlySent(thankyouKey(id)) {
		err = c.sendAutoResponse(jid, id, thankyouResponse)

		c.r.MarkRecentlySent(thankyouKey(id), time.Minute*30)
	}

	if err != nil {
		log.Print(err)
	}

	if !c.r.HasRecentlySent(disclaimerKey(id)) {
		err = c.sendAutoResponse(jid, id, disclaimerResponse)
		if err != nil {
			log.Print(err)
		}

		c.r.MarkRecentlySent(disclaimerKey(id), time.Hour*24)
	}
}

func (c Client) sendAutoResponse(jid types.JID, id, txt string) (err error) {
	ctx := context.Background()

	_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
		Conversation: stringRef(txt),
	})

	if err != nil {
		return
	}

	return c.r.Produce(id, txt)
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
