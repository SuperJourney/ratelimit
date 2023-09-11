package leaky_bucket

import (
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLeakyBucketLimiter_Request(t *testing.T) {
	t.Parallel()
	var per int64 = 1
	var unit = 100 * time.Millisecond
	rl := NewLeakyBucketLimiter(per, unit)
	var succ atomic.Int32
	log.Println(time.Now().UnixNano())
	tick := time.After(10 * time.Second)
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-tick:
				log.Println(time.Now().UnixNano())
				doneCh <- struct{}{}
				return
			default:
				// 循环结束，最后一个阻塞的线程会继续执行；所以实际预测结果会比预测结果多1
				go func() {
					if err := rl.Request(); err == nil {
						succ.Add(1)
					}
				}()
			}
		}
	}()
	<-doneCh
	assert.Equal(t, int32(101), succ.Load())
}
