# Step 2: オブジェクトストレージへの保存の実装

オブジェクトストレージにキャッシュを保存するように、実装を変更します。

## ゴール

S3 互換オブジェクトストレージにキャッシュを保存する GOCACHEPROG バックエンド実装を行い、 `go` コマンドからキャッシュが利用できる。

## 準備

S3の認証情報を環境変数で設定します。
参加者の方は、スプレッドシートの「S3認証情報」のシートを参照してください。
```bash
export S3_ENDPOINT=<エンドポイントのホスト名>
export S3_ACCESS_KEY_ID=<アクセスキーID>
export S3_SECRET_ACCESS_KEY=<シークレットアクセスキー>
export S3_SESSION_TOKEN=<セッショントークン>
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
    - オブジェクト名は `fmt.Sprintf("%s-d", escapeString(Request.OutputID))` とする
    - `Request.Body` の内容をアップロードする
2. メタデータを json 形式にエンコード
3. S3 のバケットにメタデータをアップロードする
    - オブジェクト名は `fmt.Sprintf("%s-a.json", escapeString(Request.ActionID))` とする
    - メタデータを書き込む
    - `Metadata` 構造体を使用する
    - 各フィールドの内容は以下の通り
        - `OutputID`: `Request.OutputID` と同じ値
        - `Size`: `Request.Size` と同じ値
        - `TimeNanos`: 現在の Unix ナノ時間( `time.Now().UnixNano()` )

### Step 2.2: `GetHandler` の実装
`get` リクエストを処理する `GetHandler` 関数に以下の処理を**追加**する。
> [!WARNING]
> Step1 で実装したローカルディスクへの保存の処理は残したままにしてください。レスポンスでは 
> `DiskPath` フィールドにローカルディスク上のパスを返す必要があります。

1. ローカルにメタデータファイルが存在しない場合、 S3 からメタデータを取得するように書き換える
    - Step 1 時点では `Miss` フィールドが `true` のレスポンスを返している
    - S3 からもメタデータが取得できなかった場合、 `Miss` フィールドが `true` のレスポンスを返す
    - `s3Client` の[`GetObject`](https://pkg.go.dev/github.com/minio/minio-go/v7#Client.GetObject) メソッドを使用する
2. ローカルにキャッシュデータファイルが存在しない場合、 S3 からキャッシュデータをローカルにダウンロードするように書き換える
    - Step 1 時点では `Miss` フィールドが `true` のレスポンスを返している
    - S3 からもキャッシュデータが取得できなかった場合、 `Miss` フィールドが `true` のレスポンスを返す
    - `s3Client` の[`GetObject`](https://pkg.go.dev/github.com/minio/minio-go/v7#Client.GetObject) メソッドを使用する
    - ダウンロード先は `fmt.Sprintf(".cache/%s-d", escapeString(Request.OutputID))` とする

### Step 2.3: 動作確認

1. GOCACHEPROG バックエンドをビルド(`go buiold -o ./cacheprog .`)
2. 環境変数 `GOCACHEPROG` にビルドしたバイナリのパスを設定(`export GOCACHEPROG=$PWD/cacheprog`)
    - 相対パスだと実行時ディレクトリに依存してしまうため、絶対パスでの設定推奨
3. `go build std` で GOCACHEPROG バックエンドを使用して標準ライブラリをビルド
    - Go の標準ライブラリのビルドを行っている
    - `CacheProg used.` と表示されれば GOCACHEPROG バックエンドが使用されている

### Step 2.4: (発展1) 実行時間を比較してみよう

GOCACHEPROG の有無でのビルド時間の違いを確認してみましょう。
ただし、キャッシュの有無で場合分けを行う必要があります。
また、GOCACHEPROG 有りの場合、ローカル・リモートの 2 段階のキャッシュが存在するため、以下の3パターンについて調査をする必要があります[^1]。

a. ローカルキャッシュ有り
b. ローカルキャッシュ無し、リモートキャッシュ有り
c. ローカルキャッシュ無し、リモートキャッシュ無し

これらを踏まえて、以下の手順で実行時間を比較してみましょう。

1. GOCACHEPROG なしの場合のキャッシュをクリア(`go clean -cache` )し、 `time go build std` でビルド時間を計測する
    - GOCACHEPROG なしのキャッシュ無しの場合
2. 再度`time go build std` でビルド時間を計測する
    - GOCACHEPROG なしのキャッシュ有りの場合
3. GOCACHEPROG ありの場合のローカル・リモートキャッシュをクリア(`rm -rf .cache`、`go tool s3-clean` )し、`time GOCACHEPROG=./cacheprog go build std` でビルド時間を計測する
    - GOCACHEPROG ありのローカル・リモートキャッシュ無しの場合(c)
    - `go tool s3-clean` は `tools/s3-clean` ディレクトリにある S3 互換オブジェクトストレージのバケット内のキャッシュを削除するツールです
4. GOCACHEPROG ありの場合のローカルキャッシュのみクリア(`rm -rf .cache` )し、`time GOCACHEPROG=./cacheprog go build std` でビルド時間を計測する
    - GOCACHEPROG ありのローカルキャッシュ無し、リモートキャッシュ有りの場合(b)
5. 再度`time GOCACHEPROG=./cacheprog go build std` でビルド時間を計測する
    - GOCACHEPROG ありのローカルキャッシュ有りの場合(a)

さて、 GOCACHEPROG があることで、ビルド時間は速くなったでしょうか?

[^1]: 厳密には部分的にキャッシュが存在する場合もありますが、計測が困難なためここでは考慮しないことにします。

### Step 2.5: (発展2) 速度のボトルネックを調査してみよう

GOCACHEPROG があることで、ビルド時間が逆に遅くなってしまった場合、どこにボトルネックがあるかを調査してみましょう。
既に `main.go` に pprof, trace の設定が追加されているので、これらと `htop` などのホスト本体のリソース使用率を組み合わせて調査してみましょう。

と、いきなり言われても難しいと思うので、ヒントをいくつか挙げておきます。

#### ヒント1: 計測ツールの使い方

`main.go` で取得された各種計測結果は、以下のコマンドで確認できます。

**トレース**
ヒープ使用量の変化や、ゴルーチン数の変化、システムコールの発生状況などを把握したい場合に使用します。
```bash
go tool trace metrics/trace.out
```

**CPU プロファイル**
CPU を占有している処理を把握したい場合に使用します。
```bash
go tool pprof -http=:8080 metrics/pprof/cpu.pprof
```

**fgprof**
I/O 待ち時間も含めた実行時間を計測できるツールです。
```bash
go tool pprof -http=:8080 metrics/pprof/fg.prof
```

**メモリプロファイル**
メモリを多く消費している処理を把握したい場合に使用します。
```bash
go tool pprof -http=:8080 metrics/pprof/mem.pprof
```

#### ヒント2: 調査の進め方

今回のようなパフォーマンスチューニングでは、ボトルネック(律速段階)を特定することが重要です。
ボトルネックが生まれる要因として、多いのは以下の2つです。

- ハードウェアリソース不足
    - CPU、メモリ、ディスク I/O、ネットワーク I/O などのリソースが不足している場合
    - `htop`、アクティビティモニター、タスクマネージャー などで使用率 100% になっているリソースがあるときに怪しい
- ロック・リクエスト数不足などによる処理のブロック
    - 処理上の問題で、リソースは空いているのに待ちが発生している場合
    - いずれのリソースも使用率が 100% になっていないときに怪しい
    - 処理上の特性を考慮して、見る箇所を絞り込む必要がある

これらの原因を地道に切り分けていきましょう。

1. まずは `htop` などでリソース使用率を確認する
    - いずれかのリソースが 100% に張り付いている場合、ハードウェアリソース不足が原因の可能性が高い
    - いずれのリソースも 100% に張り付いていない場合、ロック・リクエスト数不足などによる処理のブロックが原因の可能性が高い
2. 1. の結果を踏まえて、 pprof, trace のどちらを使うかを決める
    - ハードウェアリソース不足が原因の可能性が高い場合、 CPU プロファイル、メモリプロファイル、 fgprof を使う
    - ロック・リクエスト数不足などによる処理のブロックが原因の可能性が高い場合、 トレース、 fgprof を使う
        - 場合によっては `request.txt` のログを確認することも有効
