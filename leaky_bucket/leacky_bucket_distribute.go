package leaky_bucket

import (
	"context"
	"log"
	"os"
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
}

func NewDistriLeackyBucketLimiter(per int64, unit, timeOut time.Duration, redisClient *redis.Client, opt ...OptFnDistribute) *DistriLeackyBucketLimiter {
	if timeOut < unit/time.Duration(per) {
		// 超时时间小于一次请求时间无意义
		timeOut = unit / time.Duration(per)
	}
	rl := &DistriLeackyBucketLimiter{
		per:         per,
		unit:        unit,
		redisClient: redisClient,
		timeOut:     timeOut,
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

var resetScript = `
	local prefixKey = KEYS[1]
	local slackTimeKey = prefixKey .. ":slack_time"
	local lastRequestTimeKey = prefixKey .. ":last_request_timelast_request_time"
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
		local waitRKey = prefixKey .. ":wait_r"
		local timeNow = tonumber(ARGV[1])
		local per = tonumber(ARGV[2])
		local unit = tonumber(ARGV[3])
		local timeOut = tonumber(ARGV[4])
		
		local slackTime = tonumber(redis.call('GET', slackTimeKey) or 0)
		local lastRequestTime = tonumber(redis.call('GET', lastRequestTimeKey) or 0)
		local waitR = tonumber(redis.call('GET', waitRKey) or 0)

		local waitRTime = waitR * unit / per

		local exp = 3 * timeOut / 1000000000 > 1 and math.floor(3 * timeOut / 1000000000) or 1

		-- timeout must large then unit/per 
		if waitRTime > timeOut then
			return -1
		else 
			redis.call('SETEX', waitRKey, exp, waitR + 1 )
		end

		-- First request
		if lastRequestTime == 0 then
			redis.call('SETEX', lastRequestTimeKey, exp,timeNow )
			return 0
		end
		
		local nextRequestTime = lastRequestTime + (unit / per) if slackTime > 0 then
			nextRequestTime = nextRequestTime - slackTime
		end

		if slackTime > unit / per then 
			slackTime = slackTime - (unit / per)
		end

		slackTime = slackTime + (timeNow - lastRequestTime)

		if slackTime > unit then
			slackTime = unit
		end
		
		local sleepDuration = nextRequestTime - timeNow
		if sleepDuration > 0 then
			timeNow = timeNow + sleepDuration
		end
	
		redis.call('SETEX', slackTimeKey, exp, slackTime)
		redis.call('SETEX', lastRequestTimeKey,exp, timeNow)
		return sleepDuration > 0 and sleepDuration or 0
`

func (rl *DistriLeackyBucketLimiter) getPrefixKey() string {
	if rl.prefix == "" {
		rl.prefix = "ratelimit"
	}
	return rl.prefix + ":lecky_bucket_distri"
}

// 高危操作,释放会导致其他容器消费进度异常
func (rl *DistriLeackyBucketLimiter) UnsafeRelease(ctx context.Context) error {
	keys := []string{rl.getPrefixKey()}
	_, err := rl.redisClient.Eval(ctx, resetScript, keys).Result()
	return err
}

func (rl *DistriLeackyBucketLimiter) Request(ctx context.Context) error {

	keys := []string{rl.getPrefixKey()}
	args := []interface{}{
		time.Now().UnixNano(), // timeNow
		rl.per,                // per
		int64(rl.unit),        // unit
		int64(rl.timeOut),     // timeOut
	}

	sleepDuration, err := rl.redisClient.Eval(ctx, requestScript, keys, args).Int64()
	if err != nil {
		log.Println("script err:", err)
		return err
	}
	if sleepDuration < 0 {
		log.Println("time out")
		return ErrTimeOut
	}

	if sleepDuration > 0 {
		log.Println("sleep:", sleepDuration)
		time.Sleep(time.Duration(sleepDuration))
	}
	rl.redisClient.Decr(ctx, rl.getPrefixKey()+":wait_r")
	rl.redisClient.Expire(ctx, rl.getPrefixKey()+":wait_r", 5*time.Second)
	return nil
}
