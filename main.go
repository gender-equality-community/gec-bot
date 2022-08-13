package main

import (
	"log"

	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	logLevel = "DEBUG"
	testJID  = must(types.ParseJID(os.Getenv("GEC_JID")))
)

func must(in types.JID, err error) types.JID {
	if err != nil {
		panic(err)
	}

	return in
}

func main() {
	log.Print("aa")

	dbLog := waLog.Stdout("Database", logLevel, true)

	log.Print("aa")

	container, err := sqlstore.New("sqlite3", "file:.gec.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	log.Print("aa")

	r, err := NewRedis("localhost:6379", "gec")
	if err != nil {
		panic(err)
	}

	log.Print("aa")

	client, err := New(container, r)
	if err != nil {
		panic(err)
	}

	log.Print("aa")

	for {
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.disconnect()
}
