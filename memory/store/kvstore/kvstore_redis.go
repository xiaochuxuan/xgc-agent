package kvstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisKVStore implements KVStore backed by Redis.
type RedisKVStore struct {
	client *redis.Client
}

func NewRedisKVStore(client *redis.Client) *RedisKVStore {
	return &RedisKVStore{client: client}
}

func (s *RedisKVStore) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrKeyNotFound
	}
	return val, err
}

func (s *RedisKVStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisKVStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisKVStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}

var _ KVStore = (*RedisKVStore)(nil)
