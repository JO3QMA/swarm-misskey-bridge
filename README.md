# Swarm Misskey Integration

Swarmでのチェックイン情報を自動的に取得し、指定したMisskeyインスタンスに投稿するCloudflare Workersアプリケーションです。

## 機能

- **Swarmとの連携**: SwarmのAPIポーリングでチェックイン情報を取得（5分間隔）
- **Misskeyへの投稿**: チェックイン情報をMisskeyに自動投稿
- **画像対応**: Swarmに添付された画像をMisskeyにアップロード
- **カスタマイズ可能**: 投稿内容のテンプレートをカスタマイズ可能
- **セキュア**: APIキーはCloudflare WorkersのSecretsで管理
- **自動ポーリング**: Cloudflare WorkersのCron Triggers機能で自動実行
- **重複防止**: 同じチェックインの重複投稿を防止

## 技術スタック

- **言語**: Golang (WebAssembly)
- **プラットフォーム**: Cloudflare Workers
- **開発環境**: Devcontainer
- **ストレージ**: Cloudflare KV (設定と最終チェックイン時刻の保存)

## セットアップ

### 1. 前提条件

- [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/install-and-update/) がインストールされていること
- Cloudflareアカウントがあること
- MisskeyインスタンスのAPIキーがあること
- Foursquare (Swarm) のAPIキーがあること

### 2. プロジェクトのセットアップ

```bash
# リポジトリをクローン
git clone <repository-url>
cd swarm-misskey

# 依存関係をインストール
go mod tidy

# Wranglerでログイン
wrangler login
```

### 3. 設定

#### 3.1 KV Namespaceの作成

```bash
# KV namespaceを作成
wrangler kv:namespace create "CONFIG"
wrangler kv:namespace create "CONFIG" --preview
```

#### 3.2 wrangler.tomlの更新

作成されたKV namespaceのIDを`wrangler.toml`に設定してください：

```toml
[[kv_namespaces]]
binding = "CONFIG"
id = "your-kv-namespace-id"
preview_id = "your-preview-kv-namespace-id"
```

#### 3.3 Secretsの設定

```bash
# MisskeyのAPIキーを設定
wrangler secret put MISSKEY_API_KEY

# SwarmのAPIキーを設定
wrangler secret put SWARM_API_KEY

# SwarmのユーザーIDを設定
wrangler secret put SWARM_USER_ID
```

#### 3.4 環境変数の設定

```bash
# MisskeyインスタンスのURLを設定
wrangler secret put MISSKEY_INSTANCE

# 投稿テンプレートを設定（オプション）
wrangler secret put POST_TEMPLATE

# 投稿の可視性を設定（オプション）
wrangler secret put VISIBILITY
```

### 4. デプロイ

```bash
# 開発環境にデプロイ
wrangler dev

# 本番環境にデプロイ
wrangler deploy
```

## 使用方法

### ポーリング機能

このアプリケーションは、Cloudflare WorkersのCron Triggers機能を使用して5分ごとにSwarm APIをポーリングし、新しいチェックインがあればMisskeyに投稿します。

#### 手動ポーリング

テスト目的で手動でポーリングを実行できます：

```bash
curl -X POST https://your-worker.your-subdomain.workers.dev/manual-poll
```

#### 設定の確認

現在の設定を確認できます：

```bash
curl https://your-worker.your-subdomain.workers.dev/config
```

#### 設定の更新

設定を更新できます：

```bash
curl -X POST https://your-worker.your-subdomain.workers.dev/config \
  -H "Content-Type: application/json" \
  -d '{
    "misskeyInstance": "https://your-misskey-instance.com",
    "postTemplate": "Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey",
    "visibility": "public",
    "pollingInterval": 5
  }'
```

### エンドポイント一覧

| エンドポイント | メソッド | 説明 |
|---------------|---------|------|
| `/` | GET | アプリケーションの状態確認 |
| `/health` | GET | ヘルスチェック |
| `/poll` | POST | Cron Triggers用のポーリングエンドポイント |
| `/manual-poll` | POST | 手動ポーリング実行 |
| `/config` | GET | 設定の取得 |
| `/config` | POST | 設定の更新 |

## 投稿テンプレート

投稿テンプレートでは以下のプレースホルダーが使用できます：

- `{venueName}`: チェックインした場所の名前
- `{comment}`: チェックイン時のコメント
- `{url}`: SwarmのチェックインURL

例：
```
Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey
```

## トラブルシューティング

### よくある問題

1. **APIキーが正しく設定されていない**
   - `wrangler secret list` で設定を確認
   - 必要に応じて再設定

2. **KVストレージにアクセスできない**
   - `wrangler.toml` のKV namespace設定を確認
   - KV namespaceが正しく作成されているか確認

3. **ポーリングが動作しない**
   - Cloudflare Workersのログを確認
   - 手動ポーリングエンドポイントでテスト

### ログの確認

```bash
# リアルタイムログの確認
wrangler tail

# 特定の時間のログを確認
wrangler tail --format=pretty
```

## 開発

### ローカル開発

```bash
# 開発サーバーの起動
wrangler dev

# テストの実行
go test ./...
```

### テスト

```bash
# 単体テストの実行
go test -v

# カバレッジの確認
go test -cover
```

## ライセンス

MIT License
