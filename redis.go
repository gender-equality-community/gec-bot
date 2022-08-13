package main

import (
	"context"

	"github.com/go-redis/redis/v9"
)

type Redis struct {
	client *redis.Client
	stream string
}

func NewRedis(addr, stream string) (r Redis, err error) {
	r.stream = stream
	r.client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	return r, nil
}

func (r Redis) Produce(msg string) (err error) {
	return r.client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: r.stream,
		Values: []string{
			"msg",
			msg,
		},
	}).Err()
}
