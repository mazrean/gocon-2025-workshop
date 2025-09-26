package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	s3Client   *minio.Client
	bucketName string
	maxWorkers int = 10 // デフォルト並列度
)

func init() {
	endpoint, ok := os.LookupEnv("S3_ENDPOINT")
	if !ok {
		panic("S3_ENDPOINT environment variable is required")
	}

	accessKeyID, ok := os.LookupEnv("S3_ACCESS_KEY_ID")
	if !ok {
		panic("S3_ACCESS_KEY_ID environment variable is required")
	}

	secretAccessKey, ok := os.LookupEnv("S3_SECRET_ACCESS_KEY")
	if !ok {
		panic("S3_SECRET_ACCESS_KEY environment variable is required")
	}

	session, ok := os.LookupEnv("S3_SESSION_TOKEN")
	if !ok {
		panic("S3_SESSION_TOKEN environment variable is required")
	}

	bucketName, ok = os.LookupEnv("S3_BUCKET_NAME")
	if !ok {
		panic("S3_BUCKET_NAME environment variable is required")
	}

	// 並列度設定（オプション）
	if maxWorkersStr, exists := os.LookupEnv("MAX_WORKERS"); exists {
		if workers, err := strconv.Atoi(maxWorkersStr); err == nil && workers > 0 {
			maxWorkers = workers
		}
	}

	var err error
	s3Client, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, session),
		Secure: true,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create S3 client: %v", err))
	}
}

// deleteWorker はオブジェクトを並列削除するワーカー関数
func deleteWorker(ctx context.Context, objectChan <-chan minio.ObjectInfo, wg *sync.WaitGroup, deletedCount, errorCount *int64) {
	defer wg.Done()

	for object := range objectChan {
		err := s3Client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("Failed to delete %s: %v", object.Key, err)
			atomic.AddInt64(errorCount, 1)
		} else {
			atomic.AddInt64(deletedCount, 1)
		}
	}
}

// displayStats はリアルタイムで削除統計を表示
func displayStats(deletedCount, errorCount *int64, total int64, startTime time.Time, done <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			deleted := atomic.LoadInt64(deletedCount)
			errors := atomic.LoadInt64(errorCount)
			processed := deleted + errors
			elapsed := time.Since(startTime)

			var rate float64
			if elapsed.Seconds() > 0 {
				rate = float64(deleted) / elapsed.Seconds()
			}

			progress := float64(processed) / float64(total) * 100
			fmt.Printf("\r[%.1f%%] Deleted: %d | Errors: %d | Rate: %.1f obj/s",
				progress, deleted, errors, rate)
		}
	}
}

func main() {
	// コンテキストにタイムアウトを設定（デフォルト30分）
	timeoutStr := os.Getenv("CLEANUP_TIMEOUT")
	timeout := 30 * time.Minute
	if timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = t
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// バケット内のすべてのオブジェクトをリスト
	fmt.Printf("Listing objects in bucket: %s\n", bucketName)

	objectCh := s3Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	var objectsToDelete []minio.ObjectInfo
	for object := range objectCh {
		if object.Err != nil {
			panic(fmt.Sprintf("Error listing objects: %v", object.Err))
		}
		objectsToDelete = append(objectsToDelete, object)
		fmt.Printf("Found object: %s (size: %d bytes)\n", object.Key, object.Size)
	}

	if len(objectsToDelete) == 0 {
		fmt.Println("No objects found in bucket")
		return
	}

	fmt.Printf("Found %d objects to delete\n", len(objectsToDelete))

	// 確認プロンプト
	fmt.Print("Are you sure you want to delete all objects? (y/N): ")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		panic(fmt.Sprintf("Failed to read response: %v", err))
	}

	if response != "y" && response != "Y" {
		fmt.Println("Operation cancelled")
		return
	}

	// 並列削除処理開始
	fmt.Printf("Deleting objects with %d workers...\n", maxWorkers)
	startTime := time.Now()

	// 統計カウンター
	var deletedCount int64
	var errorCount int64
	totalObjects := int64(len(objectsToDelete))

	// オブジェクト配信チャンネル
	objectChan := make(chan minio.ObjectInfo, maxWorkers*2)

	// ワーカーの完了を待つ
	var wg sync.WaitGroup

	// ワーカープール開始
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go deleteWorker(ctx, objectChan, &wg, &deletedCount, &errorCount)
	}

	// 統計表示goroutine
	done := make(chan struct{})
	go displayStats(&deletedCount, &errorCount, totalObjects, startTime, done)

	// オブジェクトをワーカーに送信
	for _, object := range objectsToDelete {
		objectChan <- object
	}
	close(objectChan)

	// 全ワーカーの完了を待機
	wg.Wait()
	close(done)

	// 最終統計表示
	elapsed := time.Since(startTime)

	fmt.Printf("\n\n=== Deletion Summary ===\n")
	fmt.Printf("Total objects: %d\n", totalObjects)
	fmt.Printf("Successfully deleted: %d\n", deletedCount)
	if errorCount > 0 {
		fmt.Printf("Failed to delete: %d\n", errorCount)
	}
	fmt.Printf("Processing time: %.2fs\n", elapsed.Seconds())
}
