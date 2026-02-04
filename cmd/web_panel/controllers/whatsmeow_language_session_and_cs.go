package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// GetUserLang fetches a user's preferred language from Redis
// GetUserLang retrieves the language preference for a user identified by the given jid from Redis.
// It returns the language as a string and an error if the operation fails.
// If the language is not set for the user, it returns an empty string and a nil error.
func GetUserLang(jid string) (string, error) {
	val, err := rdb.Get(context.Background(), "user:lang:"+jid).Result()
	if err == redis.Nil {
		return "", nil // Not set yet
	}
	return val, err
}

// SetUserLang sets a user's preferred language in Redis
// SetUserLang sets the language preference for a user identified by the given jid.
// The language is stored in Redis with a key formatted as "user:lang:<jid>" and an expiration of 24 hours.
// Returns an error if the operation fails.
func SetUserLang(jid, lang string) error {
	// Set with expiration, e.g. 30 days, or 0 for no expiration
	return rdb.Set(
		context.Background(),
		"user:lang:"+jid,
		lang,
		// time.Duration(config.WebPanel.Get()().Whatsmeow.RedisExpiry)*time.Hour,
		7*24*time.Hour, // 7 days
	).Err()
}

// SetUserUseAIRafy sets a flag in Redis indicating whether the user identified by jid is using the AI Rafy feature.
// The flag is stored as "1" for true and "0" for false, with a key formatted as "user:use_ai_rafy:<jid>".
// The entry expires after 7 days. Returns an error if the operation fails.
func SetUserUseAIRafy(jid string, useAI bool) error {
	value := "0"
	if useAI {
		value = "1"
	}
	return rdb.Set(
		context.Background(),
		"user:use_ai_rafy:"+jid,
		value,
		7*24*time.Hour, // 7 days
	).Err()
}

// GetUserUseAIRafy fetches the AI Rafy usage flag for a user identified by jid from Redis.
// It returns the value and whether it's set.
// If the flag is not set, it returns false, false, nil. If set to "1", true, true, nil; if "0", false, true, nil.
// Any Redis error encountered is returned.
func GetUserUseAIRafy(jid string) (bool, bool, error) {
	val, err := rdb.Get(context.Background(), "user:use_ai_rafy:"+jid).Result()
	if err == redis.Nil {
		return false, false, nil // Not set yet
	}
	if err != nil {
		return false, false, err
	}
	return val == "1", true, nil
}

// setClientConnecting sets the clientConnecting flag to indicate whether the client is currently connecting.
// It acquires a lock on clientConnMutex to ensure thread-safe access to the clientConnecting variable.
func setClientConnecting(connecting bool) {
	clientConnMutex.Lock()
	defer clientConnMutex.Unlock()
	clientConnecting = connecting
}

// isClientConnecting returns true if the client is currently in the process of connecting.
// It safely accesses the clientConnecting variable using a mutex to ensure thread safety.
func isClientConnecting() bool {
	clientConnMutex.Lock()
	defer clientConnMutex.Unlock()
	return clientConnecting
}

// Handler CS Session Interaction
// StartCSSession initializes a new customer service (CS) session for the given userJID.
// It stores the session data in Redis with a status of "active", the current start time,
// and the user's JID. The session data is set to expire after one hour.
// Returns an error if the operation fails.
func StartCSSession(userJID string) error {
	key := fmt.Sprintf("cs_session:%s", userJID)
	data := map[string]interface{}{
		"status":   "active",
		"start_at": time.Now().Format(time.RFC3339),
		"user_jid": userJID,
	}
	if err := rdb.HSet(context.Background(), key, data).Err(); err != nil {
		return err
	}
	return rdb.Expire(context.Background(), key, time.Hour).Err()
}

// IsCSSessionActive checks if a customer service (CS) session is currently active for the given userJID.
// It queries the Redis database for the session status associated with the userJID.
// Returns true if the session status is "active", false otherwise.
// If the session does not exist (redis.Nil), it returns false with no error.
// Any other Redis error is returned as the error value.
func IsCSSessionActive(userJID string) (bool, error) {
	key := fmt.Sprintf("cs_session:%s", userJID)
	status, err := rdb.HGet(context.Background(), key, "status").Result()
	if err == redis.Nil {
		return false, nil
	}
	return status == "active", err
}

// EndCSSession ends the customer service session for the specified user by deleting
// the corresponding session key from Redis. It takes the user's JID as input and
// returns an error if the operation fails.
func EndCSSession(userJID string) error {
	key := fmt.Sprintf("cs_session:%s", userJID)
	return rdb.Del(context.Background(), key).Err()
}
