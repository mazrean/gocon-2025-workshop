package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

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
	return nil, nil
}

// TODO: Step 1.2: GetHandlerを実装しよう
// TODO: Step 2.2: GetHandlerでS3から取得するようにしよう
func GetHandler(req *Request) (*Response, error) {
	return nil, nil
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
