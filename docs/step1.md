# Step 1: ディスクの利用

`go` コマンドからのリクエストを受け取り、レスポンスを返す、最低限動作する GOCACHEPROG 実装を行います。
実装例: [`step-1` ブランチ](https://github.com/mazrean/gocon-2025-workshop/tree/step-1)

## ゴール

ローカルディスクにキャッシュを保存する GOCACHEPROG バックエンド実装を行い、 `go` コマンドからキャッシュが利用できる。

## 手順

`cacheprog.go` に実装を進めていきます。
現状、リクエストのパースとレスポンスの書き込みの処理は `processRequest` 関数に実装されています。
ここから、 `put` リクエストを処理する `PutHandler` 関数と、 `get` リクエストを処理する `GetHandler` 関数を実装します。
アプリケーション起動時に `.cache` ディレクトリが初期化されるようになっている( `storage.go` 参照 )ので、ここにキャッシュを保存するように実装していきましょう。

> [!WARNING]
>`OutputID` や `ActionID` には `/` が含まれる場合があるため、ファイル名で使用する場合はエスケープする必要があります。
>このサンプルコードでは、 `escapeString` 関数を用意しているので、これを使用してください。

### Step 1.1: `PutHandler` の実装
`put` リクエストを処理する `PutHandler` 関数を実装します。

1. `.cache` ディレクトリに `fmt.Sprintf("%s-d", escapeString(req.OutputID))` という名前でファイルを作成し、キャッシュデータ(`Request.Body`) の内容を書き込む
2. メタデータを json 形式にエンコード
3. `.cache` ディレクトリに `fmt.Sprintf("%s-a.json", escapeString(req.ActionID))` という名前でファイルを作成し、メタデータを書き込む
    - `Metadata` 構造体を使用する
    - 各フィールドの内容は以下の通り
        - `OutputID`: `Request.OutputID` と同じ値
        - `Size`: `Request.Size` と同じ値
        - `TimeNanos`: 現在の Unix ナノ時間( `time.Now().UnixNano()` )
4. `Response` を返す
    - `ID`: `Request.ID` と同じ値
    - `OutputID`: `Request.OutputID` と同じ値
    - `Size`: `Request.Size` と同じ値
    - `TimeNanos`: `Metadata.TimeNanos` と同じ値
    - `DiskPath`: 書き込んだキャッシュデータのディスク上のパス(`fmt.Sprintf(".cache/%s-d", escapeString(req.OutputID))`)
    - その他のフィールドはゼロ値でよい

### Step 1.2: `GetHandler` の実装

1. メタデータファイル(`fmt.Sprintf(".cache/%s-a.json", escapeString(req.OutputID))`) ファイルを `os.Open` する
    - 存在しない場合、 `Miss` フィールドが `true` の `Response` を返す
        - `ID`: `Request.ID` と同じ値
        - `Miss`: true
        - その他のフィールドはゼロ値でよい
2. メタデータを json デコード
3. キャッシュデータファイル(`fmt.Sprintf(".cache/%s-d", escapeString(req.OutputID))`) を　`os.Stat` で存在確認する
    - 存在しない場合、 `Miss` フィールドが `true` の `Response` を返す
        - `ID`: `Request.ID` と同じ値
        - `Miss`: true
        - その他のフィールドはゼロ値でよい
3. レスポンスを返す
    - `ID`: `Request.ID` と同じ値
    - `Miss`: false
    - `OutputID`: `Metadata.OutputID` と同じ値
    - `Size`: `Metadata.Size` と同じ値
    - `TimeNanos`: `Metadata.TimeNanos` と同じ値
    - `DiskPath`: キャッシュデータのディスク上のパス(`fmt.Sprintf(".cache/%s-d", escapeString(req.OutputID))`)
    - その他のフィールドはゼロ値でよい

### Step 1.3: 動作確認

1. GOCACHEPROG バックエンドをビルド(`go buiold -o ./cacheprog .`)
2. `GOCACHEPROG=./cacheprog go build std` で GOCACHEPROG バックエンドを使用して標準ライブラリをビルド
    - Go の標準ライブラリのビルドを行っている
    - `CacheProg used.` と表示されれば GOCACHEPROG バックエンドが使用されている

### Step 1.4: (発展1) リクエスト・レスポンスを見てみよう

GOCACHEPROG バックエンドの標準入力・標準出力が`stdin.txt`・`stdout.txt`に記録されています。
これらを見て、 `go` コマンドと GOCACHEPROG バックエンドの間でどのようなリクエスト・レスポンスがやり取りされているかを確認してみましょう。

### Step 1.5: (発展2) リクエストのパースの処理を読んでみよう

`cacheprog.go` の `processRequest` 関数では、リクエストの読み込み・レスポンスの書き込みの処理を並行で行う実装がされています。
この関数の実装を読んで、リクエストのパースとレスポンスの書き込みの処理を理解してみましょう。
また、複数のリクエストを無駄なく並行で処理するための工夫も確認してみましょう。
