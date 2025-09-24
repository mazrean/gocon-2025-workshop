package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

func CacheProg(stdin io.Reader, stdout io.Writer) error {
	err := processRequest(
		stdin, stdout,
		GetHandler, PutHandler,
	)
	if err != nil {
		return err
	}

	return nil
}

// TODO: Step 1.1: PutHandlerを実装しよう
// TODO: Step 2.1: PutHandlerでS3に保存するようにしよう
func PutHandler(req *Request) (*Response, error) {
	// 1. キャッシュデータファイルの作成と書き込み
	dataFilePath := filepath.Join(".cache", fmt.Sprintf("%s-d", escapeString(req.OutputID)))
	dataFile, err := os.Create(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache data file: %w", err)
	}
	defer dataFile.Close()

	// キャッシュデータ(Request.Body)を書き込み
	_, err = io.Copy(dataFile, req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write cache data: %w", err)
	}

	// 2. メタデータの作成
	currentTime := time.Now().UnixNano()
	metadata := Metadata{
		OutputID:  req.OutputID,
		Size:      req.BodySize,
		TimeNanos: currentTime,
	}

	// 3. メタデータファイルの作成と書き込み
	metadataFilePath := filepath.Join(".cache", fmt.Sprintf("%s-a.json", escapeString(req.ActionID)))
	metadataFile, err := os.Create(metadataFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metadataFile.Close()

	// メタデータをJSONエンコード
	encoder := json.NewEncoder(metadataFile)
	if err := encoder.Encode(metadata); err != nil {
		return nil, fmt.Errorf("failed to encode metadata: %w", err)
	}

	// 4. S3にオブジェクトを保存（S3クライアントが利用可能な場合のみ）
	if s3Client != nil && bucketName != "" {
		// キャッシュデータをS3にアップロード
		_, err = dataFile.Seek(0, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek data file: %w", err)
		}

		objectName := fmt.Sprintf("%s-d", escapeString(req.OutputID))
		_, err = s3Client.PutObject(context.Background(), bucketName, objectName, dataFile, req.BodySize, minio.PutObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to upload cache data to S3: %w", err)
		}

		// メタデータをS3にアップロード
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}

		metadataObjectName := fmt.Sprintf("%s-a.json", escapeString(req.ActionID))
		metadataReader := bytes.NewReader(metadataBytes)
		_, err = s3Client.PutObject(context.Background(), bucketName, metadataObjectName, metadataReader, int64(len(metadataBytes)), minio.PutObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to upload metadata to S3: %w", err)
		}
	}

	// 5. レスポンスを返す
	response := &Response{
		ID:        req.ID,
		OutputID:  req.OutputID,
		Size:      req.BodySize,
		TimeNanos: currentTime,
		DiskPath:  dataFilePath,
	}

	return response, nil
}

// TODO: Step 1.2: GetHandlerを実装しよう
// TODO: Step 2.2: GetHandlerでS3から取得するようにしよう
func GetHandler(req *Request) (*Response, error) {
	var metadata Metadata

	// 1. ローカルメタデータファイルを開く
	metadataFilePath := filepath.Join(".cache", fmt.Sprintf("%s-a.json", escapeString(req.ActionID)))
	metadataFile, err := os.Open(metadataFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// ローカルにメタデータが存在しない場合、S3から取得を試行
			if s3Client != nil && bucketName != "" {
				metadataObjectName := fmt.Sprintf("%s-a.json", escapeString(req.ActionID))
				s3Object, err := s3Client.GetObject(context.Background(), bucketName, metadataObjectName, minio.GetObjectOptions{})
				if err != nil {
					// S3からも取得できない場合はキャッシュミス
					return &Response{
						ID:   req.ID,
						Miss: true,
					}, nil
				}

				// S3からメタデータを取得してデコード
				decoder := json.NewDecoder(s3Object)
				if err := decoder.Decode(&metadata); err != nil {
					s3Object.Close()
					return nil, fmt.Errorf("failed to decode S3 metadata: %w", err)
				}
				s3Object.Close()
			} else {
				// S3クライアントが利用できない場合はキャッシュミス
				return &Response{
					ID:   req.ID,
					Miss: true,
				}, nil
			}
		} else {
			return nil, fmt.Errorf("failed to open metadata file: %w", err)
		}
	} else {
		// ローカルメタデータからデコード
		decoder := json.NewDecoder(metadataFile)
		if err := decoder.Decode(&metadata); err != nil {
			metadataFile.Close()
			return nil, fmt.Errorf("failed to decode metadata: %w", err)
		}
		metadataFile.Close()
	}

	// 2. キャッシュデータファイルの存在確認
	dataFilePath := filepath.Join(".cache", fmt.Sprintf("%s-d", escapeString(metadata.OutputID)))
	if _, err := os.Stat(dataFilePath); err != nil {
		if os.IsNotExist(err) {
			// ローカルにキャッシュデータが存在しない場合、S3からダウンロード
			if s3Client != nil && bucketName != "" {
				dataObjectName := fmt.Sprintf("%s-d", escapeString(metadata.OutputID))
				s3Object, err := s3Client.GetObject(context.Background(), bucketName, dataObjectName, minio.GetObjectOptions{})
				if err != nil {
					// S3からも取得できない場合はキャッシュミス
					return &Response{
						ID:   req.ID,
						Miss: true,
					}, nil
				}

				// S3からキャッシュデータをローカルにダウンロード
				dataFile, err := os.Create(dataFilePath)
				if err != nil {
					s3Object.Close()
					return nil, fmt.Errorf("failed to create local cache data file: %w", err)
				}

				_, err = io.Copy(dataFile, s3Object)
				dataFile.Close()
				s3Object.Close()
				if err != nil {
					return nil, fmt.Errorf("failed to download cache data from S3: %w", err)
				}
			} else {
				// S3クライアントが利用できない場合はキャッシュミス
				return &Response{
					ID:   req.ID,
					Miss: true,
				}, nil
			}
		} else {
			return nil, fmt.Errorf("failed to stat cache data file: %w", err)
		}
	}

	// 4. キャッシュヒット時のレスポンスを返す
	response := &Response{
		ID:        req.ID,
		Miss:      false,
		OutputID:  metadata.OutputID,
		Size:      metadata.Size,
		TimeNanos: metadata.TimeNanos,
		DiskPath:  dataFilePath,
	}

	return response, nil
}

// processRequest stdin からリクエストを読み込み、 handler の実行結果を stdout に書き込む。
//
// TODO: Step 1(発展2): リクエストの処理を読んでみよう
func processRequest(
	stdin io.Reader, stdout io.Writer,
	getHandler, putHandler func(*Request) (*Response, error),
) error {
	eg := &errgroup.Group{}

	resChan := make(chan *Response, 100)

	logFile, err := os.Create("request.txt")
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// レスポンス書き込み用 Goroutine
	eg.Go(func() error {
		bw := bufio.NewWriter(stdout)
		defer bw.Flush()

		encoder := json.NewEncoder(bw)
		for res := range resChan {
			// レスポンスを書き込む
			if err := encoder.Encode(res); err != nil {
				return err
			}

			// バッファをフラッシュして、標準出力に書き込む
			if err := bw.Flush(); err != nil {
				return err
			}

			fmt.Fprintf(logFile, "<- %+v\n", res)
		}
		return nil
	})

	// 最初に KnownCommands を返す
	resChan <- &Response{KnownCommands: []Cmd{CmdGet, CmdPut, CmdClose}}

	// リクエスト読み込み用 ループ
	decoder := json.NewDecoder(stdin)
DECODE_LOOP:
	for {
		// リクエストをjsonデコード
		var req Request
		if err := decoder.Decode(&req); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		fmt.Fprintf(logFile, "-> %+v\n", req)

		// コマンド別に処理
		switch req.Command {
		case CmdGet:
			// Getコマンドを処理
			eg.Go(func() error {
				res, err := getHandler(&req)
				if err != nil {
					res = &Response{ID: req.ID, Err: err.Error()}
					fmt.Fprintln(os.Stderr, "GetHandler error:", err)
				}
				if res == nil {
					res = &Response{ID: req.ID, Err: "not implemented"}
				}
				resChan <- res
				return nil
			})
		case CmdPut:
			if req.BodySize > 0 { // BodySize が 0 より大きい場合のみ Body を読み込む
				var body []byte
				err := decoder.Decode(&body)
				if err != nil {
					res := &Response{ID: req.ID, Err: fmt.Sprintf("failed to read body: %v", err)}
					resChan <- res
					fmt.Fprintln(os.Stderr, "Failed to read body:", err)
					continue
				}

				req.Body = bytes.NewReader(body)
			} else {
				req.Body = bytes.NewReader(nil)
			}

			// Putコマンドを処理
			eg.Go(func() error {
				res, err := putHandler(&req)
				if err != nil {
					res = &Response{ID: req.ID, Err: err.Error()}
					fmt.Fprintln(os.Stderr, "PutHandler error:", err)
					resChan <- res
					return nil
				}
				if res == nil {
					res = &Response{ID: req.ID, Err: "not implemented"}
				}
				resChan <- res
				return nil
			})
		case CmdClose:
			resChan <- &Response{ID: req.ID}
			close(resChan)
			break DECODE_LOOP
		}
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// Cmd is a command that can be issued to a process.
type Cmd string

const (
	// CmdGet キャッシュを取得
	CmdGet Cmd = "get"
	// CmdPut キャッシュを保存
	CmdPut Cmd = "put"
	// CmdClose キャッシュを閉じる
	CmdClose Cmd = "close"
)

// Request `go`コマンドからプロセスに送られるリクエスト。
//
// 例)
//
//	Get
//	```
//	{"ID":1,"Command":"get","ActionID":"0007e47d22497352b27560febf498e7081be04da8635b03b8c86625f194e3ce9"}
//	```
//	Put
//	```
//	{"ID":2,"Command":"put","ActionID":"0007e47d22497352b27560febf498e7081be04da8635b03b8c86625f194e3ce9","OutputID":"b0d36ad56f95415a36ac11fa2470287488dd725288f1ee65f5eb70afac1dceb6","BodySize":420}
//	"...<420 bytes of body data>"
//	```
//	Close
//	```
//	{"ID":3,"Command":"close"}
//	```
type Request struct {
	// ID リクエストのID。`go`コマンドが　Auto Incrementで振る
	ID int64

	// Command リクエストの種類（"get", "put", "close"）
	Command Cmd

	// ActionID Action の ID。
	ActionID string `json:",omitempty"`

	// OutputID Output の ID。Put のみ存在。
	OutputID string `json:",omitempty"`

	// BodySize Body のサイズ（バイト数）。Put のみ存在。
	BodySize int64 `json:",omitempty"`

	// Body
	Body io.ReadSeeker `json:"-"`
}

// DiskPath ディスク上のパス
type DiskPath = string

// Response `go`コマンドに返すレスポンス。
//
// 例)
//
//	初回起動時の KnownCommands を返すレスポンス
//	```
//	{"ID":0,"KnownCommands":["get","put","close"]}
//	```
//	Get のキャッシュヒットレスポンス
//	```
//	{"ID":1,"Miss":false,"OutputID":"b0d36ad56f95415a36ac11fa2470287488dd725288f1ee65f5eb70afac1dceb6","Size":420,"TimeNanos":123456789,"DiskPath":"/absolute/path/to/cache/file"}
//	```
//	Get のキャッシュミスレスポンス
//	```
//	{"ID":2,"Miss":true,"TimeNanos":987654321}
//	```
//	Put のレスポンス
//	```
//	{"ID":3,"OutputID":"b0d36ad56f95415a36ac11fa2470287488dd725288f1ee65f5eb70afac1dceb6","Size":420,"TimeNanos":123456789,"DiskPath":"/absolute/path/to/cache/file"}
//	```
//	Close のレスポンス
//	```
//	{"ID":4}
//	```
type Response struct {
	// ID リクエストの ID 。Request ID と同じ値
	ID int64

	// Err エラーが発生した場合にエラーメッセージを返す
	Err string `json:",omitempty"`

	// KnownCommands プロセスがサポートするコマンドの一覧。初回起動時に返す。
	KnownCommands []Cmd `json:",omitempty"`

	// Miss キャッシュのヒット・ミス。Get のみ必須。
	Miss bool `json:",omitempty"`

	// OutputID Output の ID。Get, Put のみ存在。
	OutputID string `json:",omitempty"`

	// Size Body のサイズ（バイト数）。Get, Put のみ存在。
	Size int64 `json:",omitempty"`

	// TimeNanos キャッシュが保存されたUnixナノ秒時間。Get, Put のみ存在。
	TimeNanos int64 `json:",omitempty"`

	// DiskPath データが保存されているディスク上の絶対パス
	DiskPath DiskPath `json:",omitempty"`
}
