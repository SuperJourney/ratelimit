package main

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	ratelimit "github.com/SuperJourney/ratelimit/leaky_bucket"
	"github.com/go-redis/redis/v8"
)

func main() {
	var per int64 = 4
	var unit = 100 * time.Millisecond
	var succCount int64 = 0
	before := time.Now().UnixNano()

	wait := &sync.WaitGroup{}
	// 10个协程请求
	for i := 0; i < 10; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			// 模拟分布式服务
			ctx := context.Background()
			redisClient := redis.NewClient(&redis.Options{
				Addr:     "127.0.0.1:6379",
				Password: "123",
			})
			defer redisClient.Close()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Panicln("err", err)
			}

			rl := ratelimit.NewDistriLeackyBucketLimiter(per, unit, redisClient, ratelimit.WithDistributeTimeOut(1*time.Millisecond))
			// defer func() {
			// 	err := rl.UnsafeReset(ctx)
			// 	if err != nil {
			// 		log.Panicln("err", err)
			// 	}
			// }()
			ticker := time.After(1 * time.Second)
			for {
				select {
				case <-ticker:
					log.Println("time on")
					return
				default:
					time.Sleep(20 * time.Millisecond)
					go func() {
						if err := rl.Request(context.Background()); err != nil {
						} else {
							atomic.AddInt64(&succCount, 1)
						}
					}()
				}
			}
		}()
	}
	wait.Wait()

	g := time.Now().UnixNano() - before
	log.Println("花费时长(ms):", g/int64(time.Millisecond))
	log.Println("成功数量:", succCount)
	log.Println("预计成功数量:", g/int64(unit/time.Duration(per)))
}
