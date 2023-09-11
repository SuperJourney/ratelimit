package leaky_bucket

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var ErrTimeOut = errors.New("time out")

type LeakyBucketLimiter struct {
	per     int64         // 单位时间次数
	unit    time.Duration // 单位时间
	timeOut time.Duration
	logger  *log.Logger

	lock sync.Mutex

	slackTime       time.Duration
	lastRequestTime time.Time
	waitR           int64 // 当前排队数量
}

type OptFn func(*LeakyBucketLimiter)

func NewLeakyBucketLimiter(per int64, unit time.Duration, opt ...OptFn) *LeakyBucketLimiter {
	rl := &LeakyBucketLimiter{
		per:    per,
		unit:   unit,
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	for _, o := range opt {
		o(rl)
	}
	return rl
}

func WithTimeOut(timeOut time.Duration) OptFn {
	return func(rl *LeakyBucketLimiter) {
		rl.timeOut = timeOut
	}
}

func WithLogger(logger *log.Logger) OptFn {
	return func(rl *LeakyBucketLimiter) {
		rl.logger = logger
	}
}

func (rl *LeakyBucketLimiter) Request() error {
	if err := rl.CheckTimeOut(); err != nil {
		log.Println(err)
		return err
	}

	rl.Lock()
	defer rl.Unlock()

	now := time.Now()
	if rl.lastRequestTime.IsZero() {
		rl.logger.Printf("pass frist request, now : %v", now)
		rl.lastRequestTime = now
		return nil
	}

	nextRequestTime := rl.lastRequestTime.Add(time.Duration(int64(rl.unit) / rl.per))

	if rl.slackTime >= time.Duration(int64(rl.unit)/rl.per) {
		nextRequestTime = nextRequestTime.Add(-rl.slackTime)
		rl.slackTime -= time.Duration(int64(rl.unit) / rl.per)
	}

	rl.slackTime += now.Sub(rl.lastRequestTime)
	if rl.slackTime > rl.unit {
		rl.slackTime = rl.unit
	}

	if now.Before(nextRequestTime) {
		sleepDuration := nextRequestTime.Sub(now)
		rl.logger.Printf("sleep: slackTime %d, nextRequestTime: %v, sleepDuration: %v", rl.slackTime/time.Microsecond, nextRequestTime, sleepDuration)
		time.Sleep(sleepDuration)
		now = now.Add(sleepDuration)
	} else {
		rl.logger.Printf("pass: slackTime %d, nextRequestTime: %v", rl.slackTime/time.Microsecond, nextRequestTime)
	}

	rl.lastRequestTime = now

	return nil
}

func (rl *LeakyBucketLimiter) Lock() {
	atomic.AddInt64(&rl.waitR, 1)
	rl.lock.Lock()
}
func (rl *LeakyBucketLimiter) Unlock() {
	rl.lock.Unlock()
	atomic.AddInt64(&rl.waitR, -1)
}

func (rl *LeakyBucketLimiter) CheckTimeOut() error {
	if rl.timeOut <= 0 {
		return nil
	}
	if int(rl.waitR)*int(rl.unit)/int(rl.per) > int(rl.timeOut) {
		fmt.Println(int(rl.waitR) * int(rl.unit) / int(rl.per))
		return ErrTimeOut
	}
	return nil
}
