package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	s3Client   *minio.Client
	bucketName string
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

	var err error
	s3Client, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, session),
		Secure: false,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create S3 client: %v", err))
	}
}

func main() {
	ctx := context.Background()

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

	// オブジェクトを削除
	fmt.Println("Deleting objects...")
	deletedCount := 0

	for _, object := range objectsToDelete {
		err := s3Client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("Failed to delete object %s: %v", object.Key, err)
			continue
		}
		deletedCount++
		fmt.Printf("Deleted: %s\n", object.Key)
	}

	fmt.Printf("Successfully deleted %d/%d objects\n", deletedCount, len(objectsToDelete))
}
