package counter

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type CounterEvent struct {
	Prefix string
	Key    string
	Delta  float64
}

type CounterWorker struct {
	counts     map[string]map[string]float64
	mu         sync.Mutex
	events     chan CounterEvent
	threshold  float64
	flushEvery time.Duration
	redis      redis.UniversalClient
	stopCh     chan struct{}
}

func NewCounterWorker(redis redis.UniversalClient, flushEvery time.Duration, threshold float64, bufferSize int) *CounterWorker {
	return &CounterWorker{
		counts:     make(map[string]map[string]float64),
		events:     make(chan CounterEvent, bufferSize),
		threshold:  threshold,
		flushEvery: flushEvery,
		redis:      redis,
		stopCh:     make(chan struct{}),
	}
}

func (cw *CounterWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(cw.flushEvery)
	defer ticker.Stop()

	for {
		select {
		case ev := <-cw.events:
			cw.mu.Lock()
			if _, ok := cw.counts[ev.Prefix]; !ok {
				cw.counts[ev.Prefix] = make(map[string]float64)
			}
			cw.counts[ev.Prefix][ev.Key] += ev.Delta
			val := cw.counts[ev.Prefix][ev.Key]
			cw.mu.Unlock()

			if val >= cw.threshold {
				_ = cw.flushToRedis(ctx, ev.Prefix)
			}

		case <-ticker.C:

			for prefix := range cw.counts {
				_ = cw.flushToRedis(ctx, prefix)
			}

		case <-cw.stopCh:
			for prefix := range cw.counts {
				_ = cw.flushToRedis(ctx, prefix)
			}
			return
		}
	}
}

func (cw *CounterWorker) GetInterval() time.Duration {
	return cw.flushEvery
}

func (cw *CounterWorker) Stop() {
	close(cw.stopCh)
}

func (cw *CounterWorker) Increment(prefix, key string, delta float64) {
	cw.events <- CounterEvent{
		Prefix: prefix,
		Key:    key,
		Delta:  delta,
	}
}

func (cw *CounterWorker) flushToRedis(ctx context.Context, prefix string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	data := cw.counts[prefix]

	if len(data) == 0 || cw.redis == nil {
		return nil
	}

	pipe := cw.redis.Pipeline()
	for k, v := range data {
		pipe.IncrByFloat(ctx, prefix+":"+k, v)
	}
	_, err := pipe.Exec(ctx)

	if err == nil {
		cw.counts[prefix] = make(map[string]float64)
	}

	return err
}

func (cw *CounterWorker) FlushNow(prefix string, ctx context.Context) error {
	return cw.flushToRedis(ctx, prefix)
}
