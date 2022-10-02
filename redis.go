package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	gtypes "github.com/gender-equality-community/types"
	redis "github.com/go-redis/redis/v9"
	"github.com/rs/xid"
	"github.com/sethvargo/go-diceware/diceware"
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

type redisClient interface {
	Get(context.Context, string) *redis.StringCmd
	HGet(context.Context, string, string) *redis.StringCmd
	HSet(context.Context, string, ...interface{}) *redis.IntCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
	XGroupCreate(context.Context, string, string, string) *redis.StatusCmd
	XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

type Redis struct {
	client    redisClient
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

func (r Redis) Produce(id, msg string) error {
	return r.produce(gtypes.SourceWhatsapp, id, msg)
}

func (r Redis) Autorespond(id, msg string) error {
	return r.produce(gtypes.SourceAutoresponse, id, msg)
}

func (r Redis) produce(source gtypes.Source, id, msg string) (err error) {
	return r.client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: r.outStream,
		Values: gtypes.NewMessage(source, id, msg).Map(),
	}).Err()
}

func (r Redis) Process(c chan gtypes.Message) (err error) {
	ctx := context.Background()

	err = r.client.XGroupCreate(ctx, r.inStream, groupName, "$").Err()
	if err != nil && err.Error() != busyGroupErr {
		return err
	}

	var (
		entries []redis.XStream
		msg     gtypes.Message
	)

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

		msg, err = gtypes.ParseMessage(entries[0].Messages[0].Values)
		if err != nil {
			break
		}

		c <- msg
	}

	return
}

// MarkRecentlySent marks a specific recipient as having received an auto-response
// message within a specific period.
//
// This is used to make sure things like welcome messages and disclaimers aren't
// sent as a response to every single message
func (r Redis) MarkRecentlySent(id string, duration time.Duration) {
	r.client.Set(context.Background(), id, "msg", duration)
}

// HasRecentlySent is the counter part of SetLastSent; it is used to determine
// whether or not to send an auto-response
func (r Redis) HasRecentlySent(id string) bool {
	return r.client.Get(context.Background(), id).Err() == nil
}

// JIDToID takes a JID and returns either an internal, anonymised ID, or
// an error - signifying that this JID is brand new
func (r Redis) JIDToID(jid types.JID) (id string, err error) {
	b, err := jidToBytes(jid)
	if err != nil {
		return
	}

	id, err = r.client.HGet(context.Background(), "jids", string(b)).Result()
	if err != nil {
		return
	}

	if len(id) == 0 {
		err = fmt.Errorf("expected result to contain one ID, received %#v", id)
	}

	return
}

// IDToJID takes an ID and returns the JID attached to it
func (r Redis) IDToJID(id string) (jid types.JID, err error) {
	data, err := r.client.HGet(context.Background(), "ids", id).Result()
	if err != nil {
		return
	}

	if len(data) == 0 {
		err = fmt.Errorf("expected result to contain one ID, received %#v", data)

		return
	}

	return jidFromBytes([]byte(data))
}

// MintID takes a JID, creates a new ID for it, and adds to
// redis
func (r Redis) MintID(j types.JID) (id string, err error) {
	var l []string
	for {
		l, err = diceware.Generate(3)
		if err != nil {
			return
		}

		id = strings.Join(l, "-")

		// Lazily check whether an ID is in use
		_, err = r.IDToJID(id)
		if err != nil {
			break
		}
	}

	jb, err := jidToBytes(j)
	if err != nil {
		return
	}

	ctx := context.Background()
	err = r.client.HSet(ctx, "jids", string(jb), id).Err()
	if err != nil {
		return
	}

	err = r.client.HSet(ctx, "ids", id, jb).Err()

	return
}

func disclaimerKey(id string) string {
	return fmt.Sprintf("disclaimer:%s", id)
}

func thankyouKey(id string) string {
	return fmt.Sprintf("ty:%s", id)
}

func jidToBytes(jid types.JID) (b []byte, err error) {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)
	err = enc.Encode(jid)
	if err != nil {
		return
	}

	return buf.Bytes(), nil
}

func jidFromBytes(b []byte) (jid types.JID, err error) {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	err = dec.Decode(&jid)

	return
}
