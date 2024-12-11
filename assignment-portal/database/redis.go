package database

import (
    "github.com/go-redis/redis/v8"
    "log"
)

var RedisClient *redis.Client
func ConnectRedis(){
	RedisClient = redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
	_, err := RedisClient.Ping(ctx).Result()
	if err!= nil {
        log.Fatal("Failed to connect to Redis:", err)
    }
    log.Println("Connected to Redis!")
}