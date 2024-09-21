ISUNARABE13-suburi
----

- https://isunarabe.org/

を利用した素振り用リポジトリ

立てるまで
----

1. ISUNARABEに登録 > 練習作成 > ISUCON13の問題フルセットをDL
1. `cp .env.sample .env`
1. .envにて、SSH_KEY_PATHだけでも指定
1. `direnv allow`
1. `make build-cfn`
1. tmp/cloudformation.yml を利用して個人のAWS垢にスタック作成
1. 5分くらい待つ > EC2ダッシュボードで確認
1. `vim tmp/ips` にIPアドレスを記述(benchmakerは先頭に `#` をつける)
1. `make check-ssh` でSSHできるか確認(debugする場合は `make check-authorized-keys` )
1. `make show-hosts` > /etc/hosts に追記
1. https://pipe.t.isucon.pw で証明書が期限切れしてないか確認
1. (期限切れの場合) https://github.com/KOBA789/t.isucon.pw/releases から `.env` の `*_PEM_URL` を記述(2つ)
1. (期限切れの場合) `make replace-pem`
1. https://pipe.t.isucon.pw で証明書が期限切れしてないか確認

ベンチマークの実行
----

1. サイドバー > ベンチマーク > 「ベンチマーク実行」押下

(Optional)NewRelicを入れる
----

1. `.env` の `NEW_RELIC_*` を記述(3つ)
1. `make add-newrelic-user-for-mysql`
1. `make install-newrelic`
