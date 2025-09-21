# 作って理解するGOCACHEPROG (Go Conference 2025 Workshop)

講義資料: https://docs.google.com/presentation/d/1Rd-J7d_9_cByjhG1XN_F_u6xY0O3SC-wHDB2tLYKJz0/edit?usp=sharing

実装することを通して、実用に耐える GOCACHEPROG の実装方法を学ぶワークショップです。

## アジェンダ
### はじめに
目安時間: 10分(14:50-15:00)

GOCACHEPROGの概要と、ワークショップの全体の流れを説明します。

### Step 1: リクエスト処理の実装
目安時間: 30分(15:00-15:30)
手順: [./docs/step1.md](./docs/step1.md)
実装例: [`step-1`ブランチ](https://github.com/mazrean/gocon-2025-workshop/tree/step-1)

このステップでは、`go`コマンドからのリクエストの処理を実装します。
これを通して`go`コマンドとGOCACHEPROGバックエンドの間でやり取りされるリクエストとレスポンスの形式を理解します。

### Step 2: オブジェクトストレージへの保存の実装
目安時間: 30分(15:30-16:00)
手順: [./docs/step2.md](./docs/step2.md)
実装例: [`step-2`ブランチ](https://github.com/mazrean/gocon-2025-workshop/tree/step-2)

このステップでは、オブジェクトストレージを使用してキャッシュを保存する方法を実装します。
これを通して、マシン間でのキャッシュの共有の実装への解像度を高めます。

### Step 3: 高速化方法の検討
目安時間: 20分(16:00-16:20)
議論の進め方: [./docs/step3.md](./docs/step3.md)

このステップでは、GOCACHEPROGのパフォーマンスを向上させるための方法を議論します。
これを通して、実用に耐えるGOCACHEPROGの実装を行うための考え方、および必要となる工夫を理解します。
