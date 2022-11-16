package server

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

var (
	RedisPrefix   = "prio-load-balancer:"
	RedisKeyNodes = RedisPrefix + "prio-load-balancer:nodes"
)

type RedisState struct {
	RedisClient *redis.Client
}

func NewRedisState(redisURI string) (*RedisState, error) {
	redisClient := redis.NewClient(&redis.Options{Addr: redisURI})
	if err := redisClient.Get(context.Background(), "somekey").Err(); err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "redis init error")
	}
	return &RedisState{
		RedisClient: redisClient,
	}, nil
}

func (s *RedisState) SaveNodes(nodeUris []string) error {
	msg, err := json.Marshal(nodeUris)
	if err != nil {
		return err
	}
	err = s.RedisClient.Set(context.Background(), RedisKeyNodes, msg, 0).Err()
	return err
}

func (s *RedisState) GetNodes() (nodeUris []string, err error) {
	res, err := s.RedisClient.Get(context.Background(), RedisKeyNodes).Result()
	if err != nil {
		if err == redis.Nil {
			return nodeUris, nil
		}
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &nodeUris)
	if err != nil {
		return nil, err
	}

	return nodeUris, nil
}
