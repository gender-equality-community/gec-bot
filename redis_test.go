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
	xrgErr   bool
	msgCount int
	msg      string
	idExists bool
	noJID    bool
	ts       any
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
	var err error
	if r.xrgErr {
		err = fmt.Errorf("read group error")
	}

	if r.msgCount > 0 {
		// Hang if we've already sent a message
		for {
		}
	}

	r.msgCount++

	var ts any
	switch r.ts {
	case nil:
		ts = 1661618790

	default:
		ts = r.ts
	}

	return redis.NewXStreamSliceCmdResult([]redis.XStream{
		{
			Messages: []redis.XMessage{
				{
					Values: map[string]interface{}{
						"id":  "abc123",
						"ts":  ts,
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
	for _, test := range []struct {
		name      string
		rc        *dummyRedis
		expect    gtypes.Message
		expectErr bool
	}{
		{"Happy path", new(dummyRedis), gtypes.Message{
			ID:        "abc123",
			Timestamp: 1661618790,
			Message:   "hello, world!",
		}, false},
		{"XGroupCreate errors bubble up", &dummyRedis{err: true}, gtypes.Message{}, true},
		{"XGroupRead errors bubble up", &dummyRedis{xrgErr: true}, gtypes.Message{}, true},
		{"Parse errors bubble up", &dummyRedis{ts: "foobarbaz"}, gtypes.Message{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := make([]gtypes.Message, 0)
			c := make(chan gtypes.Message)

			defer close(c)

			go func() {
				for m := range c {
					messages = append(messages, m)
				}
			}()

			go func() {
				err := Redis{client: test.rc}.Process(c)
				if err == nil && test.expectErr {
					t.Errorf("expected error")
				} else if err != nil && !test.expectErr {
					t.Errorf("unexpected error: %+v", err)
				}
			}()

			// Sleep for a tenth of a second to let messages chan
			// pick up message and do what it needs
			time.Sleep(time.Millisecond * 50)

			if !test.expectErr {
				if len(messages) != 1 {
					t.Fatalf("expected 1 messages, received %d", len(messages))
				}

				if !reflect.DeepEqual(test.expect, messages[0]) {
					t.Errorf("expected %#v, received %#v", test.expect, messages[0])
				}
			}
		})
	}
}
