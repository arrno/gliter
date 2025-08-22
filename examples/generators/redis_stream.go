package generators

// Commented out because we don't include dependencies

// import (
// 	"fmt"
// 	"log"
// 	"time"

// 	"github.com/go-redis/redis/v8"
// 	"golang.org/x/net/context"
// )

// type RedisGenerator struct {
// 	client    *redis.Client
// 	pageSize  int64
// 	streamKey string
// 	lastID    string
// 	done      <-chan any
// }

// func NewRedisGenerator(pageSize int64, streamKey string, client *redis.Client, done <-chan any) func() ([]map[string]any, bool, error) {
// 	if client == nil {
// 		panic("Nil client")
// 	}
// 	gen := &RedisGenerator{
// 		client:    client,
// 		pageSize:  pageSize,
// 		streamKey: streamKey,
// 		done:      done,
// 	}
// 	return gen.fetch
// }

// func (g *RedisGenerator) fetch() ([]map[string]any, bool, error) {

// 	// The only way to exit
// 	select {
// 	case <-g.done:
// 		return nil, false, nil
// 	default:
// 	}

// 	// Fetch the next batch of messages from the stream
// 	ctx := context.Background()
// 	messages, err := g.client.XRead(ctx, &redis.XReadArgs{
// 		Streams: []string{g.streamKey, g.lastID},
// 		Block:   0, // block indefinitely
// 		Count:   g.pageSize,
// 	}).Result()

// 	if err != nil {
// 		if err == redis.Nil {
// 			// No new messages in the stream, continue
// 			// This should never happen if blocking indefinitely on read
// 			return []map[string]any{}, true, nil
// 		}
// 		return nil, false, err
// 	}

// 	var results []map[string]any
// 	for _, message := range messages {
// 		for _, xMessage := range message.Messages {
// 			messageData := make(map[string]any)
// 			for key, value := range xMessage.Values {
// 				messageData[key] = value
// 			}
// 			results = append(results, messageData)
// 			g.lastID = xMessage.ID
// 		}
// 	}

// 	return results, true, nil

// }

// func main() {
// 	client := redis.NewClient(&redis.Options{
// 		Addr: "localhost:6379", // Redis server address
// 	})

// 	done := make(chan any)
// 	go func() {
// 		<-time.After(time.Duration(5) * time.Minute)
// 		close(done)
// 	}()

// 	// Create the generator function
// 	gen := NewRedisGenerator(10, "myStream", client, done)

// 	// Example of consuming the stream
// 	for {
// 		data, hasData, err := gen()
// 		if err != nil {
// 			log.Fatal("Error reading from Redis stream: ", err)
// 		}
// 		if !hasData {
// 			// No more data in the stream
// 			break
// 		}

// 		// Process the received data
// 		fmt.Println("Received data:", data)
// 	}
// }
