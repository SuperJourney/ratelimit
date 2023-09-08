package main

import (
	"fmt"
	ratelimit "ratelimit/leaky_bucket"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	rl := ratelimit.NewLeakyBucketLimiter(10, 1*time.Millisecond)
	before := time.Now().UnixNano()
	a := &sync.WaitGroup{}
	var errCount int64 = 0
	for i := 0; i < 100; i++ {
		a.Add(1)
		go func() {
			defer a.Done()
			if err := rl.Request(); err != nil {
				atomic.AddInt64(&errCount, 1)
			}
		}()
	}
	a.Wait()
	g := time.Now().UnixNano() - before
	fmt.Println(g / int64(time.Millisecond)) // 120
	fmt.Println(errCount)
}
