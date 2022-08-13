package main

import (
	"context"
	"os"

	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
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

	clientLog := waLog.Stdout("Client", logLevel, true)

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

	err := c.r.Produce(msg.Message.GetConversation())
	if err != nil {
		panic(err)
	}

	if msg.Info.Sender == testJID {
		// c.c.SendMessage(context.Background(), msg.Info.Sender, "", &waProto.Message{
		//	Conversation: stringRef("Thank you for your message, the GEC will do the thing"),
		// })
	}
}

func stringRef(s string) *string {
	return &s
}
