# twNetMap

[English](file:///Users/ymi/prj/twsnmp/twNetMap/README.md)

スキャンしたデータからネットワークマップを自動生成する、AI搭載ネットワーク検出ツールです。Go、Wails v2、およびSvelteで構築されています。

---

## 主な機能

1. **アクティブ＆パッシブネットワークスキャン**
   - **Pingチェック**: ICMPを用いてホストの到達性を確認します（非特権UDP ping、OSネイティブコマンドへのフォールバック対応）。
   - **ARPテーブル解析**: ローカルシステムおよびアクティブなSNMPエージェントからIP-MACマッピングテーブルを自動的に抽出します。
   - **ポートスキャン**: 一般的なTCPポート（21, 22, 23, 25, 80, 110, 143, 161, 443, 3306, 3389, 5432, 8080, 9100）をスキャンします。
   - **SNMPクエリ (v2c/v3)**: リモートエージェントに対してクエリを実行し、システム情報（`sysName`、`sysDesc`）、物理MACアドレス、およびLLDP（Link Layer Discovery Protocol）ネイバー情報を取得します。
   - **サービスバナー取得**: オープンポートに接続してSSH/FTPバナーをキャプチャし、HTMLレスポンスのタイトル等をパースしてクリーンアップします。

2. **AI駆動トポロジー推論**
   - [langchaingo](file:///Users/ymi/prj/twsnmp/twNetMap/go.mod#L8) を使用して、複数のLLMプロバイダー（**Ollama**、**OpenAI**、**Google Gemini**）と連携します。
   - デバイスタイプを標準カテゴリ（`router`、`switch`、`wifi`、`mobile`、`pc`、`server`、`printer`、`unknown`）に分類します。
   - LLDPトポロジー情報などの構造的推論を活用し、デバイス間のリンク関係を自動的に構築します。
   - **フィードバックループ**: ユーザーによる手動の編集（ノード情報の修正やリンクの削除）を履歴データとして保存し、次回のAIプロンプトに優先反映することで、推論の精度をユーザーの好みに適合させていきます。

3. **インタラクティブなネットワークマップ可視化**
   - `vis-network` を使用して、マップを動的かつレスポンシブに描画します。
   - ユーザーは手動でノードやリンクの追加、編集、削除を行うことができます。
   - ノードのドラッグ＆ドロップによるレイアウト調整や、自動再配置を実行可能です。

4. **充実したデータエクスポート機能**
   - **画像/ドキュメント**: PNG、SVG、PDF
   - **図面**: Draw.io (`.drawio`)
   - **データ**: JSON形式のマップデータ、JSON形式のスキャン生結果、CSV形式のノードリスト、Excelドキュメント (`.xlsx`)

---

## 技術スタック

- **バックエンド (Go)**
  - アプリケーションフレームワーク: [Wails v2](https://wails.io) (v2.12.0)
  - データベース: [bbolt](https://github.com/etcd-io/bbolt)（組み込みキーバリューストア）
  - LLM連携: [langchaingo](https://github.com/tmc/langchaingo)
  - SNMPクライアント: [gosnmp](https://github.com/gosnmp/gosnmp)
  - エクスポートライブラリ: [gopdf](https://github.com/signintech/gopdf), [excelize](https://github.com/xuri/excelize)
- **フロントエンド (Svelte & CSS)**
  - UIライブラリ: Svelte 5
  - ビルドシステム: Vite
  - スタイル: Tailwind CSS 3
  - 可視化: `vis-network`

---

## プロジェクト構成

- [main.go](file:///Users/ymi/prj/twsnmp/twNetMap/main.go): Wailsアプリケーションを起動するデスクトップ版のエントリーポイント。
- [app.go](file:///Users/ymi/prj/twsnmp/twNetMap/app.go): コアデータベース操作、スキャン制御、AIロジック、およびファイルダイアログを公開するWailsバインディングメソッド群。
- `backend/`:
  - [ai/ai.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/ai/ai.go): システム/ユーザーLLMプロンプトの構築、およびプロバイダー認証（Gemini、OpenAI、Ollama）の処理。
  - [scanner/scanner.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/scanner/scanner.go): IP範囲の解析、ICMP/Ping、TCPポートスキャン、SNMPウォーク、およびバナー取得の実行。
  - [datastore/db.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/datastore/db.go): スキャン結果、ノード設定、ユーザー編集履歴を管理するローカル `bbolt` バケットの操作。
- `frontend/`:
  - `src/App.svelte`: レイアウトとページルーティングを管理するルートビュー。
  - `src/routes/`:
    - `NetworkMap.svelte`: ノード/リンクを表示し、マップに対する操作を処理する可視化画面。
    - `NodeList.svelte`: 検出されたデバイスをリスト/テーブル形式で編集できる画面。
    - `ScanSettings.svelte` / `AISettings.svelte`: スキャン対象やAIプロバイダー等の管理設定画面。

---

## 開始方法

### 前提条件
- Go 1.26.5 以上
- Node.js (および npm)
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### 開発モードでの実行
ホットリロードを有効にして、デバッグモードでアプリケーションを起動します：
```bash
wails dev
```

### プロダクションビルドの作成
お使いのOS向けに、スタンドアロンのプロダクション実行可能バイナリをコンパイルします：
```bash
wails build
```
