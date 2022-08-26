package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/rs/xid"
	"go.mau.fi/whatsmeow/types"
)

const groupName = "gec-bot"

var (
	// Because all redis errors are of proto.RedisError it's hard to
	// do a proper error comparison.
	//
	// Instead, the best we can do is compare error strings
	busyGroupErr = "BUSYGROUP Consumer Group name already exists"
)

type Redis struct {
	client    *redis.Client
	inStream  string
	outStream string
	id        string
}

func NewRedis(addr, inStream, outStream string) (r Redis, err error) {
	r.inStream = inStream
	r.outStream = outStream

	r.client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	r.id = xid.New().String()

	return r, nil
}

func (r Redis) Produce(id, msg string) (err error) {
	return r.client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: r.outStream,
		Values: map[string]interface{}{
			"id":  id,
			"ts":  time.Now().Unix(),
			"msg": msg,
		},
	}).Err()
}

func (r Redis) Process(c chan Message) (err error) {
	ctx := context.Background()

	err = r.client.XGroupCreate(ctx, r.inStream, groupName, "$").Err()
	if err != nil && err.Error() != busyGroupErr {
		return err
	}

	var entries []redis.XStream
	for {
		entries, err = r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: r.id,
			Streams:  []string{r.inStream, ">"},
			Count:    1,
			Block:    0,
			NoAck:    false,
		}).Result()
		if err != nil {
			break
		}

		msg := entries[0].Messages[0].Values

		c <- Message{
			ID:      msg["id"].(string),
			Ts:      msg["ts"].(string),
			Message: msg["msg"].(string),
		}
	}

	return nil
}

func (r Redis) SetID(id string) {
	r.client.Set(context.Background(), id, "msg", time.Minute*30)
}

func (r Redis) IDExists(id string) bool {
	return r.client.Get(context.Background(), id).Err() == nil
}

func (r Redis) storeJID(jid types.JID, id string) (err error) {
	var b bytes.Buffer

	enc := gob.NewEncoder(&b)
	err = enc.Encode(jid)
	if err != nil {
		return
	}

	return r.client.SAdd(context.Background(), jidKey(id), string(b.Bytes())).Err()
}

func (r Redis) readJID(id string) (jid types.JID, err error) {
	members, err := r.client.SMembers(context.Background(), jidKey(id)).Result()
	if err != nil {
		return
	}

	if len(members) != 1 {
		return
	}

	dec := gob.NewDecoder(bytes.NewBufferString(members[0]))
	dec.Decode(&jid)

	return
}

func jidKey(id string) string {
	return fmt.Sprintf("jid:%s", id)
}
