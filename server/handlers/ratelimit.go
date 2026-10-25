package handlers

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	count int
	reset time.Time
}

// rateLimiter 是单进程内的固定窗口限流器。个人站点通常只跑一个实例，
// 这样不需要额外依赖；多实例部署时可以再换成 Redis 实现。
type rateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*rateWindow
	lastSweep time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: map[string]*rateWindow{}, lastSweep: time.Now()}
}

func (l *rateLimiter) Allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	if key == "" || limit <= 0 || window <= 0 {
		return true, 0
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(now)

	entry := l.entries[key]
	if entry == nil || !now.Before(entry.reset) {
		l.entries[key] = &rateWindow{count: 1, reset: now.Add(window)}
		return true, 0
	}
	if entry.count >= limit {
		retry := time.Until(entry.reset)
		if retry < time.Second {
			retry = time.Second
		}
		return false, retry
	}
	entry.count++
	return true, 0
}

func (l *rateLimiter) Reset(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (l *rateLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for key, entry := range l.entries {
		if !now.Before(entry.reset) {
			delete(l.entries, key)
		}
	}
}

func userRateKey(prefix string, id uint) string {
	return prefix + ":user:" + strconv.FormatUint(uint64(id), 10)
}

func ipRateKey(prefix, ip string) string {
	return prefix + ":ip:" + ip
}

func tooMany(c *gin.Context, retry time.Duration) {
	seconds := int(retry.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
}
