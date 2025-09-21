package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Metadata struct {
	ActionID  string `json:",omitempty"`
	ObjectID  string `json:",omitempty"`
	Size      int64  `json:",omitempty"`
	TimeNanos int64  `json:",omitempty"`
}

// Disk

func diskSaveMetadata(actionID string, metadata *Metadata) error {
	f, err := rootFs.Create(fmt.Sprintf("%s-a", actionID))
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	return encoder.Encode(metadata)
}

func diskLoadMetadata(actionID string) (*Metadata, error) {
	f, err := rootFs.Open(fmt.Sprintf("%s-a", actionID))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var metadata Metadata
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func diskSaveObject(objectID string, reader io.Reader) (DiskPath, error) {
	f, err := rootFs.Create(fmt.Sprintf("%s-o", objectID))
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", err
	}

	return DiskPath(f.Name()), nil
}

func diskGetObjectPath(objectID string) (DiskPath, bool) {
	path := fmt.Sprintf("%s-o", objectID)
	if _, err := rootFs.Stat(path); err != nil {
		return "", false
	}

	// 絶対パスを返す
	absPath, _ := filepath.Abs(filepath.Join("cache", path))
	return DiskPath(absPath), true
}

var rootFs *os.Root

func init() {
	if err := os.MkdirAll("cache", 0755); err != nil {
		panic(err)
	}

	var err error
	rootFs, err = os.OpenRoot("cache")
	if err != nil {
		panic(err)
	}
}

// S3

func s3SaveMetadata(actionID string, metadata *Metadata) error {
	// S3にメタデータを保存する処理を実装
	if s3Client == nil {
		return fmt.Errorf("S3 client is not initialized")
	}

	// メタデータをJSONにエンコード
	pr, pw := io.Pipe()
	encoder := json.NewEncoder(pw)
	go func() {
		defer pw.Close()
		if err := encoder.Encode(metadata); err != nil {
			pw.CloseWithError(err)
		}
	}()

	// S3にアップロード
	_, err := s3Client.PutObject(
		context.Background(), bucketName, fmt.Sprintf("%s-a", actionID), pr,
		-1, minio.PutObjectOptions{ContentType: "application/json"},
	)
	if err != nil {
		return err
	}

	return nil
}

func s3LoadMetadata(actionID string) (*Metadata, error) {
	if s3Client == nil {
		return nil, fmt.Errorf("S3 client is not initialized")
	}

	// S3からオブジェクトを取得
	obj, err := s3Client.GetObject(
		context.Background(), bucketName, fmt.Sprintf("%s-a", actionID), minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	var metadata Metadata
	decoder := json.NewDecoder(obj)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func s3SaveObject(objectID string, reader io.Reader) (DiskPath, error) {
	// S3にオブジェクトを保存する処理を実装
	if s3Client == nil {
		return "", fmt.Errorf("S3 client is not initialized")
	}

	// S3にアップロード
	_, err := s3Client.PutObject(
		context.Background(), bucketName, fmt.Sprintf("%s-o", objectID), reader,
		-1, minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	if err != nil {
		return "", err
	}

	return DiskPath(fmt.Sprintf("s3://%s/%s-o", bucketName, objectID)), nil
}

func s3GetObjectPath(objectID string) (io.Reader, bool) {
	// S3からオブジェクトのパスを取得する処理を実装
	if s3Client == nil {
		return nil, false
	}

	// S3にオブジェクトの存在を確認
	obj, err := s3Client.GetObject(
		context.Background(), bucketName, fmt.Sprintf("%s-o", objectID), minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, false
	}
	defer obj.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, false
	}

	return buf, true
}

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
		Secure: false,
	})
	if err != nil {
		panic(err)
	}
}
