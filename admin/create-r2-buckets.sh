#!/bin/bash

# Cloudflare R2 バケット & 一時認証情報 一括生成スクリプト
# 使用方法: ./create-r2-buckets.sh <count> [bucket-prefix]

set -euo pipefail

# 色付きログ出力
log_info() { echo -e "\e[32m[INFO]\e[0m $*" >&2; }
log_warn() { echo -e "\e[33m[WARN]\e[0m $*" >&2; }
log_error() { echo -e "\e[31m[ERROR]\e[0m $*" >&2; }

# 使用方法表示
show_usage() {
    cat << EOF
使用方法:
  $0 <count> [bucket-prefix]

引数:
  count          作成するバケット数（必須）
  bucket-prefix  バケット名のプレフィックス（オプション、デフォルト: 'r2-bucket'）

環境変数（必須）:
  CLOUDFLARE_API_TOKEN    Cloudflare APIトークン（R2:Edit権限）
  CLOUDFLARE_EMAIL        Cloudflareアカウントのメールアドレス
  CLOUDFLARE_GLOBAL_KEY   Cloudflare Global API Key
  CLOUDFLARE_ACCOUNT_ID   CloudflareアカウントID
  R2_PARENT_ACCESS_KEY    R2の親アクセスキーID（事前にUIで作成）

環境変数（オプション）:
  TTL_SECONDS            一時認証情報の有効期限（秒）デフォルト: 3600
  OUTPUT_FORMAT          出力形式 (json|csv) デフォルト: json

例:
  export CLOUDFLARE_API_TOKEN="your-api-token"
  export CLOUDFLARE_EMAIL="your@email.com"
  export CLOUDFLARE_GLOBAL_KEY="your-global-key"
  export CLOUDFLARE_ACCOUNT_ID="your-account-id"
  export R2_PARENT_ACCESS_KEY="your-parent-access-key"
  $0 5 workshop-cache
EOF
}

# 引数チェック
if [[ $# -eq 0 ]] || [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

# パラメータ取得
COUNT="$1"
BUCKET_PREFIX="${2:-r2-bucket}"
TTL_SECONDS="${TTL_SECONDS:-3600}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-json}"

# 入力値検証
if ! [[ "$COUNT" =~ ^[1-9][0-9]*$ ]]; then
    log_error "無効なカウント値: $COUNT（正の整数を指定してください）"
    exit 1
fi

if [[ "$COUNT" -gt 100 ]]; then
    log_error "カウント値が大きすぎます: $COUNT（最大100まで）"
    exit 1
fi

# 環境変数チェック
required_vars=(
    "CLOUDFLARE_API_TOKEN"
    "CLOUDFLARE_EMAIL"
    "CLOUDFLARE_GLOBAL_KEY"
    "CLOUDFLARE_ACCOUNT_ID"
    "R2_PARENT_ACCESS_KEY"
)

missing_vars=()
for var in "${required_vars[@]}"; do
    if [[ -z "${!var:-}" ]]; then
        missing_vars+=("$var")
    fi
done

if [[ ${#missing_vars[@]} -gt 0 ]]; then
    log_error "以下の環境変数が設定されていません:"
    printf '  %s\n' "${missing_vars[@]}" >&2
    echo >&2
    show_usage
    exit 1
fi

# API基本設定
CLOUDFLARE_API_BASE="https://api.cloudflare.com/client/v4"
BUCKET_API="$CLOUDFLARE_API_BASE/accounts/$CLOUDFLARE_ACCOUNT_ID/r2/buckets"
TEMP_CRED_API="$CLOUDFLARE_API_BASE/accounts/$CLOUDFLARE_ACCOUNT_ID/r2/temp-access-credentials"

# 一意なタイムスタンプ生成
TIMESTAMP=$(date +%s)

# 結果配列
declare -a results=()

# バケット作成と認証情報生成
log_info "バケット作成開始: $COUNT個のバケットを作成します"

for ((i=1; i<=COUNT; i++)); do
    bucket_name="${BUCKET_PREFIX}-$(printf "%02d" $i)"

    log_info "[$i/$COUNT] バケット作成中: $bucket_name"

    # バケット作成
    bucket_response=$(curl -s -X POST "$BUCKET_API" \
        -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$bucket_name\"}")

    # バケット作成結果チェック
    if ! echo "$bucket_response" | jq -e '.success == true' >/dev/null 2>&1; then
        log_error "バケット作成失敗: $bucket_name"
        echo "$bucket_response" | jq '.' >&2
        continue
    fi

    log_info "[$i/$COUNT] 一時認証情報生成中: $bucket_name"

    # 一時認証情報作成
    cred_response=$(curl -s -X POST "$TEMP_CRED_API" \
        -H "X-Auth-Email: $CLOUDFLARE_EMAIL" \
        -H "X-Auth-Key: $CLOUDFLARE_GLOBAL_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"bucket\": \"$bucket_name\",
            \"parentAccessKeyId\": \"$R2_PARENT_ACCESS_KEY\",
            \"permission\": \"object-read-write\",
            \"ttlSeconds\": $TTL_SECONDS
        }")

    # 認証情報作成結果チェック
    if ! echo "$cred_response" | jq -e '.success == true' >/dev/null 2>&1; then
        log_error "認証情報作成失敗: $bucket_name"
        echo "$cred_response" | jq '.' >&2
        continue
    fi

    # 結果抽出
    access_key_id=$(echo "$cred_response" | jq -r '.result.accessKeyId')
    secret_access_key=$(echo "$cred_response" | jq -r '.result.secretAccessKey')
    session_token=$(echo "$cred_response" | jq -r '.result.sessionToken')

    # 結果保存
    if [[ "$OUTPUT_FORMAT" == "csv" ]]; then
        results+=("$bucket_name,$access_key_id,$secret_access_key,$session_token")
    else
        result_json=$(jq -n \
            --arg bucket "$bucket_name" \
            --arg access_key "$access_key_id" \
            --arg secret_key "$secret_access_key" \
            --arg session_token "$session_token" \
            --arg ttl_seconds "$TTL_SECONDS" \
            '{
                bucket: $bucket,
                accessKeyId: $access_key,
                secretAccessKey: $secret_key,
                sessionToken: $session_token,
                ttlSeconds: ($ttl_seconds | tonumber),
                createdAt: (now | strftime("%Y-%m-%dT%H:%M:%SZ"))
            }')
        results+=("$result_json")
    fi

    log_info "[$i/$COUNT] 完了: $bucket_name"
done

# 結果出力
log_info "すべての処理が完了しました。結果を出力します。"

if [[ "$OUTPUT_FORMAT" == "csv" ]]; then
    echo "bucket_name,access_key_id,secret_access_key,session_token"
    printf '%s\n' "${results[@]}"
else
    # JSON配列として出力
    printf '[%s]' "$(IFS=,; echo "${results[*]}")" | jq '.'
fi

log_info "処理完了: ${#results[@]}個のバケットと認証情報を生成しました"