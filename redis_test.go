package main

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	gtypes "github.com/gender-equality-community/types"
	"github.com/go-redis/redis/v9"
)

type dummyRedis struct {
	err      bool
	msgCount int
	msg      string
	idExists bool
	noJID    bool
}

func (r dummyRedis) getErr() (err error) {
	if r.err {
		err = fmt.Errorf("an error")
	}

	return
}

func (r dummyRedis) Get(context.Context, string) *redis.StringCmd {
	var err error
	if !r.idExists {
		err = fmt.Errorf("an error")
	}

	return redis.NewStringResult("foo", err)
}

func (r dummyRedis) HGet(context.Context, string, string) *redis.StringCmd {
	s := "A\xff\x81\x03\x01\x01\x03JID\x01\xff\x82\x00\x01\x05\x01\x04User\x01\x0c\x00\x01\x05Agent\x01\x06\x00\x01\x06Device\x01\x06\x00\x01\x06Server\x01\x0c\x00\x01\x02AD\x01\x02\x00\x00\x00!\xff\x82\x01\x0c447360602643\x03\x0es.whatsapp.net\x00"
	if r.noJID {
		s = ""
	}

	return redis.NewStringResult(s, r.getErr())
}

func (r dummyRedis) HSet(_ context.Context, s string, sl ...interface{}) *redis.IntCmd {
	return redis.NewIntResult(1, r.getErr())
}

func (r dummyRedis) Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
	return redis.NewStatusResult("foo", r.getErr())
}

func (r *dummyRedis) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	r.msg = a.Values.(map[string]interface{})["msg"].(string)

	return redis.NewStringResult("foo", r.getErr())
}

func (r dummyRedis) XGroupCreate(context.Context, string, string, string) *redis.StatusCmd {
	return redis.NewStatusResult("foo", r.getErr())
}

func (r *dummyRedis) XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	err := r.getErr()
	if r.msgCount > 0 {
		// Hang if we've already sent a message
		for {
		}
	}

	r.msgCount++

	return redis.NewXStreamSliceCmdResult([]redis.XStream{
		{
			Messages: []redis.XMessage{
				{
					Values: map[string]interface{}{
						"id":  "abc123",
						"ts":  1661618790,
						"msg": "hello, world!",
					},
				},
			},
		},
	}, err)
}

func TestNewRedis(t *testing.T) {
	_, err := NewRedis("example.com:6379", "test-in", "test-out")
	if err != nil {
		t.Errorf("unexpected error: %#v", err)
	}
}

func TestRedis_Process(t *testing.T) {
	messages := make([]gtypes.Message, 0)
	c := make(chan gtypes.Message)

	defer close(c)

	go func() {
		for m := range c {
			messages = append(messages, m)
		}
	}()

	go func() {
		err := Redis{client: &dummyRedis{}}.Process(c)
		if err != nil {
			t.Errorf("unexpected error %#v", err)
		}
	}()

	// Sleep for a tenth of a second to let messages chan
	// pick up message and do what it needs
	time.Sleep(time.Millisecond * 50)

	if len(messages) != 1 {
		t.Fatalf("expected 1 messages, received %d", len(messages))
	}

	expect := gtypes.Message{
		ID:        "abc123",
		Timestamp: 1661618790,
		Message:   "hello, world!",
	}

	if !reflect.DeepEqual(expect, messages[0]) {
		t.Errorf("expected %#v, received %#v", expect, messages[0])
	}
}
