package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"staff-service/internal/models"
)

// PermissionCache handles caching of staff permissions in Redis
type PermissionCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewPermissionCache creates a new permission cache instance
func NewPermissionCache(host string, port int, password string, db int, ttlSeconds int) (*PermissionCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// Return cache with nil client - will gracefully degrade to no caching
		return &PermissionCache{
			client: nil,
			ttl:    time.Duration(ttlSeconds) * time.Second,
		}, nil
	}

	return &PermissionCache{
		client: client,
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}, nil
}

// cacheKey generates a unique cache key for staff permissions
func (c *PermissionCache) cacheKey(tenantID string, vendorID *string, staffID uuid.UUID) string {
	vendorStr := "global"
	if vendorID != nil {
		vendorStr = *vendorID
	}
	return fmt.Sprintf("perms:%s:%s:%s", tenantID, vendorStr, staffID.String())
}

// Get retrieves cached permissions for a staff member
func (c *PermissionCache) Get(ctx context.Context, tenantID string, vendorID *string, staffID uuid.UUID) (*models.EffectivePermissions, error) {
	if c.client == nil {
		return nil, nil // Cache unavailable, return nil
	}

	key := c.cacheKey(tenantID, vendorID, staffID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var perms models.EffectivePermissions
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, err
	}

	return &perms, nil
}

// Set caches permissions for a staff member
func (c *PermissionCache) Set(ctx context.Context, tenantID string, vendorID *string, staffID uuid.UUID, perms *models.EffectivePermissions) error {
	if c.client == nil {
		return nil // Cache unavailable, silently skip
	}

	key := c.cacheKey(tenantID, vendorID, staffID)
	data, err := json.Marshal(perms)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, c.ttl).Err()
}

// Invalidate removes cached permissions for a staff member
func (c *PermissionCache) Invalidate(ctx context.Context, tenantID string, vendorID *string, staffID uuid.UUID) error {
	if c.client == nil {
		return nil
	}

	key := c.cacheKey(tenantID, vendorID, staffID)
	return c.client.Del(ctx, key).Err()
}

// InvalidateAll removes all cached permissions for a tenant
// Use this when roles or permissions are updated at the tenant level
func (c *PermissionCache) InvalidateAll(ctx context.Context, tenantID string) error {
	if c.client == nil {
		return nil
	}

	pattern := fmt.Sprintf("perms:%s:*", tenantID)
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}

	return nil
}

// InvalidateByRole removes all cached permissions for users with a specific role
// This is used when a role's permissions are changed
func (c *PermissionCache) InvalidateByRole(ctx context.Context, tenantID string, roleID uuid.UUID) error {
	// For simplicity, invalidate all tenant permissions when a role changes
	// A more sophisticated implementation could track which users have which roles
	return c.InvalidateAll(ctx, tenantID)
}

// Close closes the Redis connection
func (c *PermissionCache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// IsAvailable returns true if the cache is available
func (c *PermissionCache) IsAvailable() bool {
	return c.client != nil
}
