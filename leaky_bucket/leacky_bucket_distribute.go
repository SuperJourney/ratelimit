package leaky_bucket

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

type DistriLeackyBucketLimiter struct {
	prefix      string
	redisClient *redis.Client

	per     int64         // 单位时间次数
	unit    time.Duration // 单位时间
	timeOut time.Duration
	logger  *log.Logger

	isOn atomic.Int32
}

func NewDistriLeackyBucketLimiter(per int64, unit time.Duration, redisClient *redis.Client, opt ...OptFnDistribute) *DistriLeackyBucketLimiter {
	rl := &DistriLeackyBucketLimiter{
		per:         per,
		unit:        unit,
		redisClient: redisClient,
		logger:      log.New(os.Stderr, "", log.LstdFlags),
	}
	for _, o := range opt {
		o(rl)
	}
	return rl
}

type OptFnDistribute func(*DistriLeackyBucketLimiter)

func WithDistributePrefix(prefix string) OptFnDistribute {
	return func(rl *DistriLeackyBucketLimiter) {
		rl.prefix = prefix
	}
}

func WithDistributeTimeOut(timeOut time.Duration) OptFnDistribute {
	return func(rl *DistriLeackyBucketLimiter) {
		rl.timeOut = timeOut
	}
}

var resetScript = `
	local prefixKey = KEYS[1]
	local slackTimeKey = prefixKey .. ":slack_time"
	local lastRequestTimeKey = prefixKey .. ":last_request_time"
	local waitRKey = prefixKey .. ":wait_r"
	redis.call('DEL', slackTimeKey)
	redis.call('DEL', lastRequestTimeKey)
	redis.call('DEL', waitRKey)
	return 0
`
var requestScript = `
		local prefixKey = KEYS[1]
		local slackTimeKey = prefixKey .. ":slack_time"
		local lastRequestTimeKey = prefixKey .. ":last_request_time"
	
		local timeNow = tonumber(ARGV[1])
		local per = tonumber(ARGV[2])
		local unit = tonumber(ARGV[3])
		
		local slackTime = tonumber(redis.call('GET', slackTimeKey) or 0)
		local lastRequestTime = tonumber(redis.call('GET', lastRequestTimeKey) or 0)

		-- First request
		if lastRequestTime == 0 then
			redis.call('SET', lastRequestTimeKey, timeNow)
			return 0
		end
		
		local nextRequestTime = lastRequestTime + (unit / per)
		if slackTime > 0 then
			nextRequestTime = nextRequestTime - slackTime
		end
	
		slackTime = slackTime - (unit / per)
		slackTime = slackTime + (timeNow - lastRequestTime)
		if slackTime > unit then
			slackTime = unit
		end
	
		local sleepDuration = nextRequestTime - timeNow
		if sleepDuration > 0 then
			timeNow = timeNow + sleepDuration
		end
	
		redis.call('SET', slackTimeKey, slackTime)
		redis.call('SET', lastRequestTimeKey, timeNow)
		return sleepDuration > 0 and sleepDuration or 0
`

func (rl *DistriLeackyBucketLimiter) getPrefixKey() string {
	if rl.prefix == "" {
		rl.prefix = "ratelimit"
	}
	return rl.prefix + ":lecky_bucket_distri"
}

// 高危操作,会导致其他容器消费进度异常
func (rl *DistriLeackyBucketLimiter) UnsafeReset(ctx context.Context) error {
	keys := []string{rl.getPrefixKey()}
	_, err := rl.redisClient.Eval(ctx, resetScript, keys).Result()
	return err
}

func (rl *DistriLeackyBucketLimiter) Request(ctx context.Context) error {
	if rl.isOn.Load() == 1 {
		return ErrTimeOut
	}

	keys := []string{rl.getPrefixKey()}
	args := []interface{}{
		time.Now().UnixNano(), // timeNow
		rl.per,                // per
		rl.unit,               // unit
		rl.timeOut,            // timeOut
	}

	sleepDuration, err := rl.redisClient.Eval(ctx, requestScript, keys, args).Int64()
	if err != nil {
		log.Fatalln(err)
		return err
	}

	defer rl.isOn.Store(0)
	if sleepDuration > int64(rl.timeOut) {
		rl.isOn.Store(1)
	}

	if sleepDuration == 0 {
		log.Println("sleep:", sleepDuration)
		return nil
	} else {
		log.Println("sleep:", sleepDuration)
		time.Sleep(time.Duration(sleepDuration))
		return nil
	}
}
