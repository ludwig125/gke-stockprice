package main

import (
	"context"
	"log"
	"time"
)

func retry(limit int, interval time.Duration, fn func() error) error {
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

func retryContext(ctx context.Context, limit int, interval time.Duration, fn func() error) error {
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
