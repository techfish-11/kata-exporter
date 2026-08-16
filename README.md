# Kata Exporter

SwitchBot Kata Friends の状態と日記統計を Prometheus に公開する、Go製・外部依存ゼロのExporterです。GrafanaダッシュボードとLinux/systemd向けの一括インストーラーを同梱しています。

## 取れるメトリクス

- バッテリー残量、オンライン状態、ファームウェア
- Normal / Standby / Sleep モード
- Strolling / Playing / Sleeping / Returning などの現在状態
- チャイルドロック、修理・メンテナンス・清掃状態
- 指定期間内のイベント日記、AI日記、AIコミック日記の件数と最終記録時刻
- APIリクエスト数・失敗数、scrape時間、デバイス発見成否

日記本文・画像キー・イベント詳細はPrometheusへ出しません。プライバシー保護とラベルカーディナリティ抑制のため、件数と時刻だけを出します。

## 最短セットアップ（Linux + systemd）

SwitchBotアプリで「プロフィール → 設定 → アプリバージョンを10回タップ → 開発者向けオプション」からOpen TokenとSecretを取得します。

```bash
make build
sudo -E KATA_TOKEN='...' KATA_SECRET='...' ./dist/kata-exporter install
```

インストーラーは以下を行います。

1. `/usr/local/bin/kata-exporter` に自分自身を配置
2. 権限 `0600` の `/etc/kata-exporter/config.json` を生成
3. hardening済みsystemd unitを作成・起動
4. Prometheus設定をバックアップしてscrape jobを追記（検出できた場合）
5. Grafana dashboard provisioningを配置（検出できた場合）

設定ファイルを編集後は `sudo systemctl restart kata-exporter`。疎通確認は以下です。

```bash
kata-exporter check --config /etc/kata-exporter/config.json
curl http://127.0.0.1:9788/metrics
```

## 手動実行

```bash
cp config.example.json kata-exporter.json
# token / secretを編集
KATA_CONFIG=./kata-exporter.json ./kata-exporter serve
```

設定値は `KATA_TOKEN`, `KATA_SECRET`, `KATA_DEVICE_IDS`, `KATA_LISTEN`, `KATA_DIARY_ENABLED`, `KATA_DIARY_WINDOW`, `KATA_DIARY_REFRESH` などの環境変数でも上書きできます。`device_ids` が空ならアカウント内のKata Friendsを自動検出します。

## Docker Compose

```bash
cp .env.example .env
# .env の KATA_TOKEN / KATA_SECRET を実際の値に変更
docker compose up -d
```

ComposeはExporter、Prometheus、Grafanaをまとめて起動します。Prometheusは
`kata-exporter:9788` を自動的にscrapeし、GrafanaにはPrometheusデータソースと
同梱ダッシュボードが自動登録されます。

- Grafana: <http://127.0.0.1:3000>
- Prometheus UI: <http://127.0.0.1:19090>
- Exporter metrics: <http://127.0.0.1:9788/metrics>
- Prometheusのデータは名前付きボリューム `prometheus-data` に保存されます。
- Grafanaのデータは名前付きボリューム `grafana-data` に保存されます。
- 保持期間は `.env` の `PROMETHEUS_RETENTION`（デフォルト `30d`）で変更できます。
- Grafanaへは `.env` の `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` でログインします。

起動状態とscrape状態は次のコマンドで確認できます。

```bash
docker compose ps
docker compose logs -f
# Prometheus UIの Status > Target health でも確認可能
```

## ビルド・テスト

```bash
make test
make build
make cross
```

Go 1.22以上が必要です。リリース用ビルドでは `-ldflags "-X main.version=vX.Y.Z"` を指定できます。

## 注意

- SwitchBot OpenAPIは個人利用向けで、商用・大規模利用はSwitchBotへの相談が必要です。
- 日記APIの最大期間は31日です。デフォルトは24時間、15分ごとに再取得します。
- Token/Secretをコマンドライン引数に直接書くとshell historyやプロセス一覧に残り得るため、環境変数または設定ファイルを推奨します。
- Prometheus設定の自動追記時は `.kata-exporter.bak` を一度だけ作成します。

## HTTP endpoints

- `/metrics` — Prometheus metrics
- `/-/healthy` — liveness check
- `/` — landing page

## License

MIT
