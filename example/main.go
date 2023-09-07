package main

import (
	"fmt"
	"ratelimit"
)

func main() {
	rl := ratelimit.NewRateLimiter()
	for i := 0; i < 1200; i++ {
		if i%60 == 0 {
			fmt.Println("time.sleeped")
			// time.Sleep(1 * time.Second)
		}
		rl.Request()
	}
}
