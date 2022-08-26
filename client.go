package main

import (
	"context"
	"crypto/sha256"
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

type Client struct {
	c *whatsmeow.Client
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
	if c.c.Store.ID == nil {
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

	c.c.MarkRead([]types.MessageID{msg.Info.ID}, time.Now(), jid, jid)

	id := fmt.Sprintf("%x", sha256.Sum256([]byte(msg.Info.Sender.String())))
	txt := msg.Message.GetConversation()

	for _, err := range []error{
		c.r.Produce(id, txt),
		c.r.storeJID(jid, id),
	} {
		if err != nil {
			panic(err)
		}
	}

	var err error

	if isMaybeGreeting(txt) {
		_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef("Hello, and welcome to the Anonymous GEC Report Bot. What's on your mind?"),
		})
		if err != nil {
			log.Print(err)
		}

		return
	}

	// If we haven't messaged this person the standard 'thanks for your response' in the last
	// 30 minutes then do so now
	if !c.r.IDExists(id) {
		c.r.SetID(id)
		_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef("Thank you for your message, we understand how hard it is speaking out. Please provide us with all the information you can."),
		})

		if err != nil {
			log.Print(err)
		}
	}
}

func (c Client) ResponseQueue(m chan Message) {
	ctx := context.Background()

	for msg := range m {
		jid, err := c.r.readJID(msg.ID)
		if err != nil || jid.IsEmpty() {
			log.Print(err)

			continue
		}

		_, err = c.c.SendMessage(ctx, jid, "", &waProto.Message{
			Conversation: stringRef(msg.Message),
		})

		if err != nil {
			log.Print(err)
		}
	}
}

func stringRef(s string) *string {
	return &s
}
