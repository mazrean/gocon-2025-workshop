# Step 2: オブジェクトストレージへの保存の実装

オブジェクトストレージにキャッシュを保存するように、実装を変更します。

## ゴール

S3 互換オブジェクトストレージにキャッシュを保存する GOCACHEPROG バックエンド実装を行い、 `go` コマンドからキャッシュが利用できる。

## 準備

環境変数の設定をします。
```bash
export S3_ENDPOINT=<エンドポイントのホスト名>
export S3_ACCESS_KEY_ID=<アクセスキーID>
export S3_SECRET_ACCESS_KEY=<シークレットアクセスキー>
export S3_BUCKET_NAME=<バケット名>
```

## 手順

`cacheprog.go` に実装を進めていきます。
`PutHandler` 関数と`GetHandler` 関数の処理を書き換えることで、オブジェクトストレージにキャッシュを保存するようにします。
アプリケーション起動時に `s3Client` が初期化されるようになっている( `storage.go` 参照 )ので、これを使用して S3 互換オブジェクトストレージにアクセスします。
注意: s3Client は AWS Go SDK ではなく、 [MinIO Go Client SDK](https://pkg.go.dev/github.com/minio/minio-go/v7) を使用しています。API が異なるため、ドキュメントを確認する際には間違えないように注意してください。

### Step 2.1: `PutHandler` の実装
`put` リクエストを処理する `PutHandler` 関数に以下の処理を**追加**する。
注意: Step1 で実装したローカルディスクへの保存の処理は残したままにしてください。レスポンスでは `DiskPath` フィールドにローカルディスク上のパスを返す必要があります。

1. S3 のバケットにオブジェクトをアップロードする
  - オブジェクト名は `{{OutputID}}-d` とする
  - `Request.Body` の内容をアップロードする
2. メタデータを json 形式にエンコード
3. S3 のバケットにメタデータをアップロードする
  - オブジェクト名は `{{ActionID}}-a.json` とする
  - メタデータを書き込む
  - `Metadata` 構造体を使用する
  - 各フィールドの内容は以下の通り
    - `OutputID`: `Request.OutputID` と同じ値
    - `Size`: `Request.Size` と同じ値
    - `TimeNanos`: 現在の Unix ナノ時間( `time.Now().UnixNano()` )

### Step 2.2: `GetHandler` の実装
`get` リクエストを処理する `GetHandler` 関数に以下の処理を**追加**する。
注意: Step1 で実装したローカルディスクへの保存の処理は残したままにしてください。レスポンスでは `DiskPath` フィールドにローカルディスク上のパスを返す必要があります。

1. ローカルにメタデータファイルが存在しない場合、 S3 からメタデータを取得するように書き換える
  - Step 1 時点では `Miss` フィールドが `true` のレスポンスを返している
  - S3 からもメタデータが取得できなかった場合、 `Miss` フィールドが `true` のレスポンスを返す
  - `s3Client` の[`GetObject`](https://pkg.go.dev/github.com/minio/minio-go/v7#Client.GetObject) メソッドを使用する
2. ローカルにキャッシュデータファイルが存在しない場合、 S3 からキャッシュデータをローカルにダウンロードするように書き換える
  - Step 1 時点では `Miss` フィールドが `true` のレスポンスを返している
  - S3 からもキャッシュデータが取得できなかった場合、 `Miss` フィールドが `true` のレスポンスを返す
  - `s3Client` の[`GetObject`](https://pkg.go.dev/github.com/minio/minio-go/v7#Client.GetObject) メソッドを使用する
  - ダウンロード先は `.cache/{{OutputID}}-d` とする

### Step 2.3: 動作確認

1. GOCACHEPROG バックエンドをビルド(`go buiold -o ./cacheprog .`)
2. 環境変数 `GOCACHEPROG` にビルドしたバイナリのパスを設定(`export GOCACHEPROG=$PWD/cacheprog`)
  - 相対パスだと実行時ディレクトリに依存してしまうため、絶対パスでの設定推奨
3. `go build std` で GOCACHEPROG バックエンドを使用して標準ライブラリをビルド
  - Go の標準ライブラリのビルドを行っている
  - `CacheProg used.` と表示されれば GOCACHEPROG バックエンドが使用されている

### Step 2.4: (発展1) 実行時間を比較してみよう



1. `go clean -cache` 、`rm -rf ./cache` でローカルキャッシュをクリア
2. `time go build std` で GOCACHEPROG バックエンド
