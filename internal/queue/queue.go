package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const queueKey = "fileforge:jobs:pending"

type Queue struct {
	client *redis.Client
}

func New(addr string, poolSize int) (*Queue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     poolSize,
		MinIdleConns: 2,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  35 * time.Second, 
		WriteTimeout: 5 * time.Second,
	})

	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = client.Ping(ctx).Err()
		cancel()

		if err == nil {
			break
		}

		log.Printf("[queue] Redis ping attempt %d/30: %v", attempt, err)
		time.Sleep(time.Second)
	}

	if err != nil {
		client.Close()
		return nil, fmt.Errorf("redis not ready after 30 attempts: %w", err)
	}

	log.Println("[queue] Connected to Redis")
	return &Queue{client: client}, nil
}

func (q *Queue) Close() error {
	return q.client.Close()
}

func (q *Queue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

func (q *Queue) Enqueue(ctx context.Context, jobID string) error {
	if err := q.client.LPush(ctx, queueKey, jobID).Err(); err != nil {
		return fmt.Errorf("enqueue job %s: %w", jobID, err)
	}
	return nil
}

func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) (string, error) {
	result, err := q.client.BRPop(ctx, timeout, queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("dequeue: %w", err)
	}

	if len(result) < 2 {
		return "", fmt.Errorf("dequeue: unexpected result length %d", len(result))
	}

	return result[1], nil
}

func (q *Queue) Length(ctx context.Context) (int64, error) {
	n, err := q.client.LLen(ctx, queueKey).Result()
	if err != nil {
		return 0, fmt.Errorf("queue length: %w", err)
	}
	return n, nil
}

func (q *Queue) Requeue(ctx context.Context, jobID string) error {
	if err := q.client.RPush(ctx, queueKey, jobID).Err(); err != nil {
		return fmt.Errorf("requeue job %s: %w", jobID, err)
	}
	return nil
}