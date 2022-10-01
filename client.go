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

var (
	emptyMessageError = fmt.Errorf("Empty message from whatsapp")
)

type whatsappClient interface {
	AddEventHandler(handler whatsmeow.EventHandler) uint32
	GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	Connect() error
	Disconnect()
	MarkRead([]string, time.Time, types.JID, types.JID) error
	SendMessage(context.Context, types.JID, string, *waProto.Message) (whatsmeow.SendResponse, error)
	SendPresence(types.Presence) error
	SetStatusMessage(string) error
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

	err = c.c.Connect()
	if err != nil {
		return
	}

	return c.c.SetStatusMessage(statusMessage)
}

func (c *Client) disconnect() {
	c.c.Disconnect()
}

func (c Client) handler(evt interface{}) {
	// #nosec
	c.c.SendPresence(types.PresenceAvailable)

	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	}
}

func (c Client) handleMessage(msg *events.Message) {
	if msg.Info.IsFromMe {
		return
	}

	// If message came in over 15 minutes before the app booted,
	// then ignore it.
	//
	// This is important for when the app restarts and gets a new
	// database instance, such as if the underlying hardware goes away
	// or the cluster on which it runs goes to hell
	//
	// This is, in effect, a core component of our DR strategy- should
	// we bring the app back from an outage and need to re-QR then
	// we don't want to have to react to every message we've ever seen
	// or else we're just going to spam the hell out of everyone.
	//
	// The 15 minutes gives us some safety in case getting the phone and
	// QR-ing takes too long and we miss messages
	if !msg.Info.Timestamp.After(boottime.Add(0 - (15 * time.Minute))) {
		return
	}

	jid := msg.Info.Sender.ToNonAD()

	// lookup id for jid
	var (
		id  string
		err error
	)

	defer func() {
		if err != nil {
			log.Print(err)
		}
	}()

	id, err = c.r.JIDToID(jid)
	if err != nil {
		id, err = c.r.MintID(jid)
		if err != nil {
			return
		}
	}

	txt, err := readMessage(msg)
	if err != nil {
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

	if err != nil || c.r.HasRecentlySent(disclaimerKey(id)) {
		return
	}

	err = c.sendAutoResponse(jid, id, disclaimerResponse)
	if err != nil {
		return
	}

	c.r.MarkRecentlySent(disclaimerKey(id), time.Hour*24)
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

	c.r.MarkRecentlySent(thankyouKey(msg.ID), time.Minute*30)

	return
}

func stringRef(s string) *string {
	return &s
}

func readMessage(msg *events.Message) (txt string, err error) {
	if msg.Message.ExtendedTextMessage != nil {
		return msg.Message.ExtendedTextMessage.GetText(), nil
	}

	txt = msg.Message.GetConversation()

	if txt == "" {
		err = emptyMessageError
	}

	return
}
