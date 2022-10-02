package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gender-equality-community/types"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	LogLevel = "DEBUG"
	dbLog    = waLog.Stdout("Database", LogLevel, true)
	mChan    = make(chan types.Message)

	boottime time.Time
)

func init() {
	boottime = time.Now()
}

func main() {
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

	go r.Process(mChan)
	go client.ResponseQueue(mChan)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.disconnect()
}
