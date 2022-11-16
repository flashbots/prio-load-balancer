package server

import (
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/stretchr/testify/require"
)

var redisTestServer *miniredis.Miniredis
var redisTestState *RedisState

func resetTestRedis() {
	var err error
	if redisTestServer != nil {
		redisTestServer.Close()
	}

	redisTestServer, err = miniredis.Run()
	if err != nil {
		panic(err)
	}

	redisTestState, err = NewRedisState(redisTestServer.Addr())
	if err != nil {
		panic(err)
	}
}

func TestRedisStateSetup(t *testing.T) {
	var err error
	_, err = NewRedisState("localhost:18279")
	require.NotNil(t, err, err)
}

func TestRedisNodes(t *testing.T) {
	resetTestRedis()

	nodes0, err := redisTestState.GetNodes()
	require.Nil(t, err, err)
	require.Equal(t, 0, len(nodes0))

	err = redisTestState.SaveNodes([]string{"http://localhost:12431", "http://localhost:12432"})
	require.Nil(t, err, err)

	nodes2, err := redisTestState.GetNodes()
	require.Nil(t, err, err)
	require.Equal(t, 2, len(nodes2))
}
