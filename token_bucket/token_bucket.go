package Token_bucket

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

type TokenBucketLimiter struct {
	per  int64         // 单位时间次数
	unit time.Duration // 单位时间
	mu   sync.Mutex

	size            int64         // 桶容量
	token           time.Duration // 桶大小用时间来控制，线性
	lastRequestTime time.Time     // 最后一次令牌提供时间

	logger *log.Logger

	TotalToken time.Duration
}

type OptFn func(*TokenBucketLimiter)

// 固定令牌生产速度；
func NewTokenBucketLimiter(per int64, unit time.Duration, opt ...OptFn) *TokenBucketLimiter {
	rl := &TokenBucketLimiter{
		per:    per,
		unit:   unit,
		logger: log.New(os.Stderr, "", log.LstdFlags),
		size:   10,
	}

	for _, o := range opt {
		o(rl)
	}
	return rl
}

func WithLogger(logger *log.Logger) OptFn {
	return func(rl *TokenBucketLimiter) {
		rl.logger = logger
	}
}

func WithSize(size int64) OptFn {
	if size == 0 {
		panic("size must > 0")
	}
	return func(rl *TokenBucketLimiter) {
		rl.size = size
	}
}

func (rl *TokenBucketLimiter) Request() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	perUnit := rl.unit / time.Duration(rl.per) // 一次请求所需要时间
	maxToken := time.Duration(rl.size * int64(perUnit))
	if rl.lastRequestTime.IsZero() {

		log.Println("request pass,frist request")
		rl.lastRequestTime = now
		rl.token = maxToken - perUnit
		return nil
	}

	t := now.Sub(rl.lastRequestTime)

	if rl.token+t > maxToken {
		log.Println("请求太少，令牌积压了")
		rl.token = maxToken
	}

	if rl.token+t > perUnit {
		rl.token = rl.token + t - perUnit
		rl.lastRequestTime = now

		log.Println("request pass, cur token(ms): ", rl.token)
		// debug
		rl.TotalToken += perUnit
		return nil
	} else {
		// log.Println("request err: token is 0")
		return errors.New("token is 0")
	}
}
