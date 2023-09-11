package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	ratelimit "github.com/SuperJourney/ratelimit/leaky_bucket"
)

func main() {
	var per int64 = 4
	var unit = 100 * time.Millisecond
	var succCount int64 = 0
	rl := ratelimit.NewLeakyBucketLimiter(per, unit, ratelimit.WithTimeOut(50*time.Millisecond))

	before := time.Now().UnixNano()

	wait := &sync.WaitGroup{}
	// 10个协程请求
	for i := 0; i < 10; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ticker := time.After(1 * time.Second)
			for {
				select {
				case <-ticker:
					log.Println("time on")
					return
				default:
					if err := rl.Request(); err != nil {
						time.Sleep(20 * time.Millisecond)
					} else {
						atomic.AddInt64(&succCount, 1)
					}
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
