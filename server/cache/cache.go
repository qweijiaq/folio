package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 封装可选的 Redis。地址为空或连接失败时所有方法自动降级，
// 站点仍然可以只用数据库运行。
type Cache struct {
	client *redis.Client
}

func New(addr, password string, db int) *Cache {
	if addr == "" {
		log.Println("未配置 FOLIO_REDIS_ADDR，Redis 功能关闭")
		return &Cache{}
	}
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis 连接失败，降级为无缓存模式: %v", err)
		_ = client.Close()
		return &Cache{}
	}
	log.Printf("Redis 已连接: %s", addr)
	return &Cache{client: client}
}

func (c *Cache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *Cache) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Close() {
	if c.Enabled() {
		_ = c.client.Close()
	}
}

// Blacklist 把已登出的 JWT 放进黑名单，直到它自然过期。
func (c *Cache) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {
	if !c.Enabled() || jti == "" || ttl <= 0 {
		return nil
	}
	return c.client.Set(ctx, "folio:jwt:blacklist:"+jti, "1", ttl).Err()
}

func (c *Cache) IsBlacklisted(ctx context.Context, jti string) bool {
	if !c.Enabled() || jti == "" {
		return false
	}
	n, err := c.client.Exists(ctx, "folio:jwt:blacklist:"+jti).Result()
	return err == nil && n > 0
}

// MarkView 在 TTL 内只允许同一个人/同一 IP 计数一次。Redis 关闭时始终计数。
func (c *Cache) MarkView(ctx context.Context, docID uint, viewer string, ttl time.Duration) bool {
	if !c.Enabled() {
		return true
	}
	ok, err := c.client.SetNX(ctx, fmt.Sprintf("folio:view:%d:%s", docID, viewer), "1", ttl).Result()
	if err != nil {
		return true
	}
	return ok
}
