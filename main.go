package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	LogLevel  = "DEBUG"
	db        = os.Getenv("DATABASE")
	redisAddr = os.Getenv("REDIS_ADDR")

	testJID = must(types.ParseJID(os.Getenv("GEC_JID")))
)

func must(in types.JID, err error) types.JID {
	if err != nil {
		panic(err)
	}

	return in
}

func main() {
	dbLog := waLog.Stdout("Database", LogLevel, true)

	container, err := sqlstore.New("sqlite3",
		fmt.Sprintf("file:.%s?_foreign_keys=on", db),
		dbLog,
	)

	if err != nil {
		panic(err)
	}

	r, err := NewRedis(redisAddr, "gec")
	if err != nil {
		panic(err)
	}

	client, err := New(container, r)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.disconnect()
}
