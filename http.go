package httpcache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/demonoid81/krakend-httpcache/httpcache"
	"github.com/redis/go-redis/v9"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/transport/http/client"
)

type Cache interface {
	// Get returns the []byte representation of a cached response and a bool
	// set to true if the value isn't empty
	Get(key string) (responseBytes []byte, ok bool)
	// Set stores the []byte representation of a response against a key
	Set(key string, responseBytes []byte)
	// Delete removes the value associated with the key
	Delete(key string)
}

// Namespace is the key to use to store and access the custom config data
const Namespace = "github.com/demonoid81/krakend-httpcache"

var redisClient *redis.Client
var ttl time.Duration

func init() {
	redisURL, exists := os.LookupEnv("REDIS_URL")
	if exists {
		panic("Redis url is not set")
	}
	redisPassw := os.Getenv("REDIS_PASSWORD")

	redisDB := 0
	if dbs, exists := os.LookupEnv("REDIS_DB"); exists {
		db, err := strconv.Atoi(dbs)
		if err != nil {
			panic(err)
		}
		redisDB = db
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassw, // no password set
		DB:       redisDB,    // use default DB
	})

	redisClient = rdb

	ttls := os.Getenv("DEFAULT_TTL")
	ttli, err := strconv.Atoi(ttls)
	if err != nil {
		panic(err)
	}
	ttl = time.Duration(ttli) * time.Second

	fmt.Println("initializing")
}

// NewHTTPClient creates a HTTPClientFactory using an in-memory-cached http client
func NewHTTPClient(cfg *config.Backend, nextF client.HTTPClientFactory) client.HTTPClientFactory {
	raw, ok := cfg.ExtraConfig[Namespace]
	if !ok {
		return nextF
	}

	if b, err := json.Marshal(raw); err == nil {
		var opts options
		if err := json.Unmarshal(b, &opts); err == nil && opts.TTL > 0 {
			ttl = opts.TTL * time.Second
		}
	}

	cache := httpcache.NewMemoryCache(redisClient, ttl)

	return func(ctx context.Context) *http.Client {
		httpClient := nextF(ctx)
		return &http.Client{
			Transport: &httpcache.Transport{
				Transport: httpClient.Transport,
				Cache:     cache,
			},
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
			Timeout:       httpClient.Timeout,
		}
	}
}

type options struct {
	TTL time.Duration `json:"ttl"`
}
