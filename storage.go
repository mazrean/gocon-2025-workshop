package main

import (
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Metadata struct {
	OutputID  string `json:",omitempty"`
	Size      int64  `json:",omitempty"`
	TimeNanos int64  `json:",omitempty"`
}

// Disk

func init() {
	if err := os.MkdirAll(".cache", 0755); err != nil {
		panic(err)
	}
}

// S3

var (
	s3Client   *minio.Client
	bucketName string
)

func init() {
	endpoint, ok := os.LookupEnv("S3_ENDPOINT")
	if !ok {
		// S3_ENDPOINTが設定されていない場合はローカルファイルシステムを使用
		return
	}

	accessKeyID, ok := os.LookupEnv("S3_ACCESS_KEY_ID")
	if !ok {
		// S3_ACCESS_KEY_IDが設定されていない場合はローカルファイルシステムを使用
		return
	}

	secretAccessKey, ok := os.LookupEnv("S3_SECRET_ACCESS_KEY")
	if !ok {
		// S3_SECRET_ACCESS_KEYが設定されていない場合はローカルファイルシステムを使用
		return
	}

	session, ok := os.LookupEnv("S3_SESSION_TOKEN")
	if !ok {
		// S3_SESSION_TOKENが設定されていない場合はローカルファイルシステムを使用
		return
	}

	bucketName, ok = os.LookupEnv("S3_BUCKET_NAME")
	if !ok {
		// S3_BUCKET_NAMEが設定されていない場合はローカルファイルシステムを使用
		return
	}

	var err error
	s3Client, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, session),
		Secure: true,
	})
	if err != nil {
		panic(err)
	}
}
