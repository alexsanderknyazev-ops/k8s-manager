package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	loginLimiters   = make(map[string]*ipLimiter)
	loginLimitersMu sync.RWMutex
	apiLimiters     = make(map[string]*ipLimiter)
	apiLimitersMu   sync.RWMutex
)

const cleanupEvery = 2 * time.Minute

func getIP(c *gin.Context) string {
	if x := c.GetHeader("X-Forwarded-For"); x != "" {
		return x
	}
	return c.ClientIP()
}

func getLimiter(m *sync.RWMutex, store map[string]*ipLimiter, ip string, perMin int) *rate.Limiter {
	if perMin <= 0 {
		return nil
	}
	m.Lock()
	defer m.Unlock()
	if l, ok := store[ip]; ok {
		l.lastSeen = time.Now()
		return l.limiter
	}
	limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(perMin)), perMin)
	store[ip] = &ipLimiter{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

func cleanup(m *sync.RWMutex, store map[string]*ipLimiter, maxAge time.Duration) {
	m.Lock()
	defer m.Unlock()
	now := time.Now()
	for ip, l := range store {
		if now.Sub(l.lastSeen) > maxAge {
			delete(store, ip)
		}
	}
}

// RateLimitLogin ограничивает запросы на /api/login (perMin запросов в минуту с одного IP).
func RateLimitLogin(perMin int) gin.HandlerFunc {
	go func() {
		tick := time.NewTicker(cleanupEvery)
		defer tick.Stop()
		for range tick.C {
			cleanup(&loginLimitersMu, loginLimiters, 5*time.Minute)
		}
	}()
	return func(c *gin.Context) {
		if perMin <= 0 {
			c.Next()
			return
		}
		ip := getIP(c)
		limiter := getLimiter(&loginLimitersMu, loginLimiters, ip, perMin)
		if limiter != nil && !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
			return
		}
		c.Next()
	}
}

// RateLimitAPI ограничивает запросы к /api/* (perMin в минуту с одного IP). perMin=0 — без лимита.
func RateLimitAPI(perMin int) gin.HandlerFunc {
	if perMin <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	go func() {
		tick := time.NewTicker(cleanupEvery)
		defer tick.Stop()
		for range tick.C {
			cleanup(&apiLimitersMu, apiLimiters, 5*time.Minute)
		}
	}()
	return func(c *gin.Context) {
		ip := getIP(c)
		limiter := getLimiter(&apiLimitersMu, apiLimiters, ip, perMin)
		if limiter != nil && !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
