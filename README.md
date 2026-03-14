# x-screen-time-limiter-backend

[X Screen Time Limiter](https://github.com/kepitan/x-screen-time-limiter) の複数端末同期機能を提供するバックエンドです。

## 概要

PHP CGI スクリプト 1 ファイルで動作するシンプルな同期 API です。使用データを JSON ファイルに保存し、複数端末間で最大値マージによる同期を行います。

## ファイル構成

```
sync.php     # API 本体
.htaccess    # DirectoryIndex 設定
data.json    # データ保存ファイル（自動生成、git 管理外）
```

## API

### GET `/ping` (ヘルスチェック)

```
GET https://example.com/path/to/sync.php/ping
```

レスポンス:
```json
{"pong":true}
```

### POST `/sync` (データ同期)

```
POST https://example.com/path/to/sync.php/sync
Content-Type: application/json
```

リクエスト:
```json
{
  "token": "your-secret-token",
  "usage": {
    "2024-01-01": [0, 0, 0, 0, 0, 0, 0, 0, 120, 300, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
  }
}
```

- `token`: 端末を識別するための秘密のトークン（自由な文字列）
- `usage`: 日付ごとの時間帯別使用秒数（24要素の配列）

レスポンス:
```json
{
  "usage": {
    "2024-01-01": [0, 0, 0, 0, 0, 0, 0, 0, 120, 300, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
  }
}
```

サーバー上の同じトークンのデータとクライアントのデータを各時間帯の最大値でマージした結果を返します。

## セットアップ

1. `sync.php` と `.htaccess` をサーバーにアップロード
2. `sync.php` があるディレクトリが Web サーバーから書き込み可能であることを確認
3. 拡張機能の設定画面で API ベース URL を `https://example.com/path/to/sync.php` に設定

### データ保存先の変更

デフォルトでは `sync.php` と同じディレクトリに `data.json` が作成されます。変更する場合は `sync.php` の以下の行を編集してください。

```php
$DATA_FILE = __DIR__ . '/data.json';
```

## データ形式

`data.json` の形式:

```json
{
  "トークン文字列": {
    "YYYY-MM-DD": [秒, 秒, ...(24要素)]
  }
}
```

## 同期の仕組み

- 各時間帯ごとにクライアントとサーバーの値の大きい方を採用します
- これにより、ブラウザ再インストール後やオフライン期間があっても既存データが失われません
- 同じトークンを設定した端末間でデータが共有されます
