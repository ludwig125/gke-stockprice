package retry

import (
	"context"
	"log"
	"time"
)

// Retry is retry function.
func Retry(limit int, interval time.Duration, fn func() error) error {
	attempt := 1
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if attempt < limit {
			log.Printf("attempt %d failed: %v. sleep %v and retry.", attempt, err, interval)
			time.Sleep(interval)
			attempt++
			continue
		}
		log.Printf("attempt %d failed: %v. reached attempt limit %d.", attempt, err, limit)
		return err
	}
}

// WithContext is retry function with context.
func WithContext(ctx context.Context, limit int, interval time.Duration, fn func() error) error {
	attempt := 1
	for {
		select {
		case <-ctx.Done(): // ctx のcancelを受け取ったら終了
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}
		if attempt < limit {
			log.Printf("attempt %d failed: %v. sleep %v and retry.", attempt, err, interval)
			time.Sleep(interval)
			attempt++
			continue
		}
		log.Printf("attempt %d failed: %v. reached attempt limit %d.", attempt, err, limit)
		return err
	}
}
