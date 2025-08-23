# Swarm Misskey Integration

Swarmでのチェックイン情報を自動的に取得し、指定したMisskeyインスタンスに投稿するCloudflare Workersアプリケーションです。

## 機能

- **Swarmとの連携**: SwarmのWebhookまたはAPIポーリングでチェックイン情報を取得
- **Misskeyへの投稿**: チェックイン情報をMisskeyに自動投稿
- **画像対応**: Swarmに添付された画像をMisskeyにアップロード
- **カスタマイズ可能**: 投稿内容のテンプレートをカスタマイズ可能
- **セキュア**: APIキーはCloudflare WorkersのSecretsで管理

## 技術スタック

- **言語**: Golang (WebAssembly)
- **プラットフォーム**: Cloudflare Workers
- **開発環境**: Devcontainer

## セットアップ

### 1. 前提条件

- [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/install-and-update/) がインストールされていること
- Cloudflareアカウントがあること
- MisskeyインスタンスのAPIキーがあること

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

# SwarmのWebhookシークレットを設定（オプション）
wrangler secret put SWARM_WEBHOOK_SECRET
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

### Webhookエンドポイント

SwarmからWebhookを受け取る場合：

```
POST https://your-worker.your-subdomain.workers.dev/webhook
```

### ヘルスチェック

```
GET https://your-worker.your-subdomain.workers.dev/health
```

### ルートエンドポイント

```
GET https://your-worker.your-subdomain.workers.dev/
```

## 設定オプション

### 投稿テンプレート

以下のプレースホルダーが使用できます：

- `{venueName}`: チェックインした場所の名前
- `{comment}`: チェックイン時のコメント
- `{url}`: チェックインのURL

例：
```
Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey
```

### 投稿の可視性

- `public`: 公開
- `home`: ホームタイムライン
- `followers`: フォロワーのみ
- `specified`: 指定したユーザーのみ

## API仕様

### Swarm Webhookペイロード

```json
{
  "id": "checkin-id",
  "venueName": "場所の名前",
  "comment": "チェックイン時のコメント",
  "url": "https://swarmapp.com/checkin/...",
  "imageUrl": "https://example.com/image.jpg",
  "createdAt": "2024-01-01T12:00:00Z",
  "userId": "user-id"
}
```

### レスポンス

成功時：
```json
{
  "success": true,
  "message": "Checkin processed successfully"
}
```

エラー時：
```json
{
  "success": false,
  "error": "エラーメッセージ"
}
```

## 開発

### Devcontainer

このプロジェクトはDevcontainerに対応しています。VS CodeでDevcontainerを開くことで、開発環境が自動的にセットアップされます。

### ローカル開発

```bash
# 依存関係をインストール
go mod tidy

# テストを実行
go test ./...

# ローカルで実行
go run main.go
```

## トラブルシューティング

### よくある問題

1. **APIキーが無効**
   - MisskeyのAPIキーが正しく設定されているか確認
   - APIキーに適切な権限があるか確認

2. **画像のアップロードに失敗**
   - 画像URLがアクセス可能か確認
   - Misskeyインスタンスのファイルアップロード制限を確認

3. **Webhookが受信されない**
   - SwarmのWebhook設定を確認
   - エンドポイントURLが正しいか確認

### ログの確認

```bash
# リアルタイムログを確認
wrangler tail

# 特定の時間のログを確認
wrangler tail --since 2024-01-01T00:00:00Z
```

## ライセンス

MIT License

## 貢献

プルリクエストやイシューの報告を歓迎します。
