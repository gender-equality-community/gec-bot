package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	// Greeting Response is sent when a recipient sends a message that looks vaguely like
	// a greeting.
	//
	// To understand what this might look like, take a look in phrases.go
	greetingResponse = "Hello, and welcome to the Anonymous GEC Report Bot. What's on your mind?"

	// Thank You response is sent when a recipient sends us a message.
	//
	// To keep this from spamming the hell out of people, we only send a maximum of 1
	// response per 30 minutes.
	//
	// Essentiall, when  a message comes in, we check whether we've responded in the last
	// 30 minutes and if we haven't then we send another
	thankyouResponse = "Thank you for your message, we understand how hard it is speaking out. Please provide us with all the information you can."
)

var (
	LogLevel  = "DEBUG"
	db        = os.Getenv("DATABASE")
	redisAddr = os.Getenv("REDIS_ADDR")
)

func main() {
	dbLog := waLog.Stdout("Database", LogLevel, true)

	container, err := sqlstore.New("sqlite3",
		fmt.Sprintf("file:.%s?_foreign_keys=on", db),
		dbLog,
	)

	if err != nil {
		panic(err)
	}

	r, err := NewRedis(redisAddr, "gec-responses", "gec")
	if err != nil {
		panic(err)
	}

	client, err := New(container, r)
	if err != nil {
		panic(err)
	}

	mChan := make(chan Message)
	go r.Process(mChan)
	go client.ResponseQueue(mChan)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.disconnect()
}
