package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	platformredis "github.com/trandinh0506/hotel-management/backend/internal/platform/redis"
)

// acquireHoldLua atomically checks that NONE of KEYS exist, then sets ALL of KEYS
// to ARGV[1] (hold_token) with TTL ARGV[2] (seconds) using NX EX.
// Returns 1 if all keys were acquired, or 0 if any key already existed (no keys modified).
var acquireHoldLua = goredis.NewScript(`
for i = 1, #KEYS do
	if redis.call("EXISTS", KEYS[i]) == 1 then
		return 0
	end
end
local token = ARGV[1]
local ttl = tonumber(ARGV[2])
for i = 1, #KEYS do
	redis.call("SET", KEYS[i], token, "EX", ttl, "NX")
end
return 1
`)

// releaseHoldLua atomically deletes each key in KEYS ONLY if its current value matches ARGV[1] (hold_token).
// Returns the number of keys deleted.
var releaseHoldLua = goredis.NewScript(`
local token = ARGV[1]
local deleted = 0
for i = 1, #KEYS do
	if redis.call("GET", KEYS[i]) == token then
		deleted = deleted + redis.call("DEL", KEYS[i])
	end
end
return deleted
`)

// HoldStore implements atomic Redis Lua hold operations for the booking module.
type HoldStore struct {
	client *platformredis.Client
}

// NewHoldStore constructs a Redis-backed hold store.
func NewHoldStore(client *platformredis.Client) *HoldStore {
	return &HoldStore{client: client}
}

// AcquireHoldKeys atomically acquires all provided Redis hold keys with the same token and TTL.
// Returns (true, nil) if all keys were acquired, (false, nil) on key conflict, or (false, err) on Redis error.
func (s *HoldStore) AcquireHoldKeys(ctx context.Context, keys []string, token string, ttlSeconds int) (bool, error) {
	if s == nil || s.client == nil || s.client.Raw() == nil {
		return false, fmt.Errorf("redis hold store is not configured")
	}
	if len(keys) == 0 {
		return true, nil
	}
	if ttlSeconds <= 0 {
		return false, fmt.Errorf("ttlSeconds must be positive")
	}

	res, err := acquireHoldLua.Run(ctx, s.client.Raw(), keys, token, ttlSeconds).Int64()
	if err != nil {
		return false, fmt.Errorf("redis acquire hold lua: %w", err)
	}
	return res == 1, nil
}

// ReleaseHoldKeys deletes each key in keys only when its value equals token.
func (s *HoldStore) ReleaseHoldKeys(ctx context.Context, keys []string, token string) (int64, error) {
	if s == nil || s.client == nil || s.client.Raw() == nil {
		return 0, fmt.Errorf("redis hold store is not configured")
	}
	if len(keys) == 0 {
		return 0, nil
	}
	res, err := releaseHoldLua.Run(ctx, s.client.Raw(), keys, token).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis release hold lua: %w", err)
	}
	return res, nil
}

// ScanHoldKeys scans all active hold:* keys (hold:room:* and hold:group:*) and returns key -> token.
func (s *HoldStore) ScanHoldKeys(ctx context.Context) (map[string]string, error) {
	if s == nil || s.client == nil || s.client.Raw() == nil {
		return nil, fmt.Errorf("redis hold store is not configured")
	}
	rdb := s.client.Raw()
	out := make(map[string]string)

	for _, pattern := range []string{"hold:room:*", "hold:group:*"} {
		var cursor uint64
		for {
			keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				return nil, fmt.Errorf("redis scan %s: %w", pattern, err)
			}
			if len(keys) > 0 {
				vals, err := rdb.MGet(ctx, keys...).Result()
				if err != nil {
					return nil, fmt.Errorf("redis mget %s: %w", pattern, err)
				}
				for i, k := range keys {
					if i < len(vals) && vals[i] != nil {
						if str, ok := vals[i].(string); ok && strings.TrimSpace(str) != "" {
							out[k] = str
						}
					}
				}
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}
	return out, nil
}

// KeyTTL returns the remaining TTL of a Redis key (useful for verification and reconciliation).
func (s *HoldStore) KeyTTL(ctx context.Context, key string) (time.Duration, error) {
	if s == nil || s.client == nil || s.client.Raw() == nil {
		return 0, fmt.Errorf("redis hold store is not configured")
	}
	return s.client.Raw().TTL(ctx, key).Result()
}
