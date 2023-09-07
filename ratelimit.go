package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	per  int64         // 单位时间次数
	unit time.Duration // 单位时间
	lock sync.Mutex

	slackTime time.Duration

	lastRequestTime time.Time

	timeOut time.Duration
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		per:       20,
		unit:      10 * time.Millisecond,
		slackTime: 10 * time.Millisecond,

		timeOut: 500 * time.Microsecond, // 预估等待时长超过timeOut秒，直接返回失败
	}
}

func (rl *RateLimiter) Request() error {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	now := time.Now()

	// First request
	if rl.lastRequestTime.IsZero() {
		rl.lastRequestTime = now
		return nil
	}

	// Next request
	nextRequestTime := rl.lastRequestTime.Add(time.Duration(int64(rl.unit) / rl.per))

	if rl.slackTime > 0 {
		nextRequestTime = nextRequestTime.Add(-rl.slackTime)
	}

	rl.slackTime = rl.slackTime - time.Duration(int64(rl.unit)/rl.per)
	rl.slackTime = rl.slackTime + now.Sub(rl.lastRequestTime)
	if rl.slackTime > rl.unit {
		rl.slackTime = rl.unit
	}

	if nextRequestTime.After(now) {
		sleepDuration := nextRequestTime.Sub(now)
		// if sleepDuration > rl.timeOut {
		// 	fmt.Printf("timeout： slack %d, nextRequestTime:%v, sleepDuration:%v \n ", rl.slackTime, nextRequestTime, sleepDuration)
		// 	rl.lastRequestTime = now
		// 	return errors.New("timeout")
		// }
		time.Sleep(sleepDuration)
		fmt.Printf("sleep： slack %d, nextRequestTime:%v, sleepDuration:%v \n ", rl.slackTime, nextRequestTime, sleepDuration)
	} else {
		fmt.Printf("pass： slack %d, nextRequestTime:%v \n", rl.slackTime, nextRequestTime)
	}

	rl.lastRequestTime = now

	return nil
}
