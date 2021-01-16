# ローカルでの開発環境構築

## gcloudインストール

[Linux 用のクイックスタート  \|  Cloud SDK のドキュメント  |  Google Cloud](https://cloud.google.com/sdk/docs/quickstart-linux?hl=ja)

インストール（自分がインストールしたときのバージョンの例）
```
$ curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-293.0.0-linux-x86_64.tar.gz
```

アーカイブをファイル システム上に展開

- ホームディレクトリをおすすめしますと書いてあった
- 自分がWSLを使っていた時は`/mnt/c/wsl`という、Cドライブ直下に`wsl` というディレクトリをホームディレクトリにしていたが、`gcloud components update`でpermission deniedが出るという問題が解決できなかったので `/home/$USER` 以下に展開した
```
$ tar zxvf google-cloud-sdk-293.0.0-linux-x86_64.tar.gz /home/$USER/google-cloud-sdk
```

インストール スクリプトを実行して、Cloud SDK ツールをパスに追加
```
$./google-cloud-sdk/install.sh

Do you want to help improve the Google Cloud SDK (y/N)?  n

```

```
$ gcloud init
```

-> URLが表示されたので、ブラウザでそのURLを開いて許可、コードが出てくるので、
「Enter verification code:」にそれを入力した

```
gcloud config set accessibility/screen_reader true
```

## mysqlのインストール

```bash
$ sudo apt install mysql-server mysql-client

$mysql --version
mysql  Ver 14.14 Distrib 5.7.30, for Linux (x86_64) using  EditLine wrapper
```

起動に使用するmysqlユーザーのホームディレクトリが存在しないとmysql serverを立ち上げられないので/etc/passwdに以下を追加

```bash
$ sudo usermod -d /var/lib/mysql mysql
```

/etc/passwdに以下が追加されている

```bash
mysql:x:111:115:MySQL Server,,,:/var/lib/mysql:/bin/false
```

mysql server の起動

```bash
$ sudo service mysql start
```

ubuntu18.04でデフォルトのmysql5.7ではroot権限でないと接続できないらしい
これだと個人開発環境では不便なので、sudoいらなくさせる

```bash
mysql > ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '';
mysql > FLUSH PRIVILEGES;
```

これでsudoもpasswordも不要になる

## go test

mysqlが起動してればこれができる
```
$go test -v ./... -p 1 -count=1
```
go testはデフォルトではパラレルでテストを実行してしまうので、
Mysqlのデータが競合しないように`-p 1`として並列数を１にしている

## ローカルのMySQLへの接続

- devはパスワードがないので以下で接続できる

```bash
$ mysql -u root --host 127.0.0.1 --port 3306
```

ローカルでのテストでは`cleanup, err := database.SetupTestDB(3306)`のようにcleanup関数を返して、`defer cleanup()` でテスト用DBを消すようにしているが、
これを以下のようにしてテストを実行すればその時のDBがそのままみられる
```golang
  _, err := database.SetupTestDB(3306)
	// cleanup, err := database.SetupTestDB(3306)
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	// defer cleanup()
```

```
$mysql -u root --host 127.0.0.1 --port 3306
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 89
Server version: 5.7.30-0ubuntu0.18.04.1 (Ubuntu)

Copyright (c) 2000, 2020, Oracle and/or its affiliates. All rights reserved.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> show databases;
+--------------------+
| Database           |
+--------------------+
| information_schema |
| grafana_db         |
| mysql              |
| performance_schema |
| stockprice_dev     |
| sys                |
+--------------------+
6 rows in set (0.01 sec)

mysql> use stockprice_dev;
Reading table information for completion of table and column names
You can turn off this feature to get a quicker startup with -A

Database changed
mysql> show tables;
+--------------------------+
| Tables_in_stockprice_dev |
+--------------------------+
| daily                    |
| movingavg                |
| trend                    |
+--------------------------+
3 rows in set (0.00 sec)

mysql>
```

## ローカル環境からcircleciジョブの実行

CIRCLE_API_USER_TOKENを事前に環境変数に設定してからcurlを実行する

```bash
$CIRCLE_API_USER_TOKEN=$(cat circleci_token.txt); curl -XPOST https://circleci.com/api/v1.1/project/github/ludwig125/gke-stockprice/tree/master --user "${CIRCLE_API_USER_TOKEN}:" --header "Content-Type: application/json" -d '{
  "build_parameters": {
    "CIRCLE_JOB": "delete_gke_cluster_by_golang"
  }
}'
```

## circleci cli

cliのインストール(以下はWSL2 Ubuntu18.04でやった場合)

- 参考：https://circleci.com/docs/2.0/local-cli/

```
$curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh | s
udo bash

$which circleci
/usr/local/bin/circleci
```

バリデーションチェック
```
$circleci config validate -c .circleci/config.yml
```

## kustomize

インストールとパス通す
https://kubernetes-sigs.github.io/kustomize/installation/source/
```
$GOBIN=$(pwd)/ GO111MODULE=on go get sigs.k8s.io/kustomize/kustomize/v3

$cp ./kustomize /usr/local/bin/
$which kustomize
/usr/local/bin/kustomize
```

ローカル環境でkustomizeの展開を確認
```
$sh local_kustomize_check_prod.sh
```

# integration_test

事前に以下のようなSpreadsheetを用意しておく必要がある

![image](https://user-images.githubusercontent.com/18366858/100282407-e05df100-2fae-11eb-8982-17b4542a5f30.png)
![image](https://user-images.githubusercontent.com/18366858/100282419-e94ec280-2fae-11eb-98d2-c14344e660eb.png)
![image](https://user-images.githubusercontent.com/18366858/100282444-f1a6fd80-2fae-11eb-8cb2-0623d679a803.png)
![image](https://user-images.githubusercontent.com/18366858/100282459-f8ce0b80-2fae-11eb-8af6-85f795c8e5f2.png)
![image](https://user-images.githubusercontent.com/18366858/100282475-02f00a00-2faf-11eb-8675-2c699ec5ca75.png)

これらはそれぞれkustomization.yamlで指定している以下のシートに対応する
```
  - HOLIDAY_SHEETID=dev_sheetid.txt # integration_test前に配置しておく
  - COMPANYCODE_SHEETID=dev_sheetid.txt # integration_test前に配置しておく
  - TREND_SHEETID=dev_sheetid.txt # integration_test前に配置しておく
  - STATUS_SHEETID=dev_sheetid.txt # integration_test前に配置しておく
```

- unittestはsheet_test.goで指定している`INTEGRATION_TEST_SHEETID`がこのシートに対応する

# 本番CloudSQL

## instanceの作成

```bash
$ gcloud sql instances create gke-stockprice-cloudsql-prod --tier=db-f1-micro --region=us-central1 --storage-auto-increase --no-backup
```

instanceの確認
```
gcloud sql instances list
```

passwordの設定

```bash
$ gcloud sql users set-password root --host=% --instance=gke-stockprice-cloudsql-prod --prompt-for-password
Instance Password: # ここに設定したいパスワードを入力
Updating Cloud SQL user...done.
```

connectionNameの確認方法

```bash
$ gcloud sql instances describe gke-stockprice-cloudsql-prod | grep connectionName
connectionName: gke-stockprice:us-central1:gke-stockprice-cloudsql-prod
```

## Databaseの準備

cloud_sql_proxy経由で接続する

cloud_sql_proxy取得

- <https://cloud.google.com/sql/docs/mysql/connect-admin-proxy?hl=ja>

install

```
$wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O cloud_sql_proxy
$chmod +x cloud_sql_proxy
```

cloud_sql_proxyで上のconnectionNameとportを指定

- ローカルのMySQLのPort(3306)ととかぶらないように3307を使用する

認証をしていなかったら事前に認証が必要

```
 $gcloud auth activate-service-account --key-file gke-stockprice-serviceaccount.json
 ```

```bash
$ ./cloud_sql_proxy -instances=gke-stockprice:us-central1:gke-stockprice-cloudsql-prod=tcp:3307
```

別に端末を開いてMySQLコマンドで接続

```bash
$ mysql -u root --host 127.0.0.1 --port 3307 -p
Enter password:   // <- passwordを入力
```

database作成
```
CREATE DATABASE IF NOT EXISTS stockprice;
```

database確認
```
mysql> show databases;
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mysql              |
| performance_schema |
| stockprice         |
| sys                |
+--------------------+
5 rows in set (0.13 sec)

mysql>
```

table作成

daily
```bash
CREATE TABLE IF NOT EXISTS stockprice.daily (
		code VARCHAR(10) NOT NULL,
		date VARCHAR(10) NOT NULL,
		open VARCHAR(15),
		high VARCHAR(15),
		low VARCHAR(15),
		close VARCHAR(15),
		turnover VARCHAR(15),
		modified VARCHAR(15),
		PRIMARY KEY( code, date )
	);
```

movingavg
```bash
CREATE TABLE IF NOT EXISTS stockprice.movingavg (
        code VARCHAR(10) NOT NULL,
        date VARCHAR(10) NOT NULL,
        moving3 DOUBLE,
        moving5 DOUBLE,
        moving7 DOUBLE,
        moving10 DOUBLE,
        moving20 DOUBLE,
        moving60 DOUBLE,
        moving100 DOUBLE,
        PRIMARY KEY( code, date )
	);
```

trend
```bash
CREATE TABLE IF NOT EXISTS stockprice.trend (
        code VARCHAR(10) NOT NULL,
        date VARCHAR(10) NOT NULL,
        trend TINYINT(20),
        trendTurn TINYINT(10),
        growthRate DOUBLE,
        crossMoving5 TINYINT(10),
        continuationDays TINYINT(20),
        PRIMARY KEY( code, date )
	);
```

table確認
```
mysql> use stockprice
Reading table information for completion of table and column names
You can turn off this feature to get a quicker startup with -A

Database changed
mysql> show tables;
+----------------------+
| Tables_in_stockprice |
+----------------------+
| daily                |
| movingavg            |
| trend                |
+----------------------+
3 rows in set (0.13 sec)

mysql>
```

table定義確認

```
mysql> desc daily;
+----------+-------------+------+-----+---------+-------+
| Field    | Type        | Null | Key | Default | Extra |
+----------+-------------+------+-----+---------+-------+
| code     | varchar(10) | NO   | PRI | NULL    |       |
| date     | varchar(10) | NO   | PRI | NULL    |       |
| open     | varchar(15) | YES  |     | NULL    |       |
| high     | varchar(15) | YES  |     | NULL    |       |
| low      | varchar(15) | YES  |     | NULL    |       |
| close    | varchar(15) | YES  |     | NULL    |       |
| turnover | varchar(15) | YES  |     | NULL    |       |
| modified | varchar(15) | YES  |     | NULL    |       |
+----------+-------------+------+-----+---------+-------+
8 rows in set (0.14 sec)

mysql>
```

# GCR(Google Container Registry)操作

事前にdockerのインストールが必要

## dockerインストール

#### install手順

- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)

#### 実行するコマンド

```
$sudo apt-get remove docker docker-engine docker.io containerd runc

$curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -

$sudo apt-key fingerprint 0EBFCD88

$ sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

$ sudo apt-get install docker-ce docker-ce-cli containerd.io
```

#### docker daemonの起動

WindowsTerminalをWindowsキーから「管理者として実行」で起動して、

![image](https://user-images.githubusercontent.com/18366858/92527936-d73c6e00-f262-11ea-992e-6425a2e3610c.png)

以下のコマンドで実行できる

```
$sudo cgroupfs-mount
$sudo service docker start
 * Starting Docker: docker                                                                                       [ OK ]
$sudo service docker status
 * Docker is running
```

## GCR認証と操作

#### 認証

```
$gcloud auth activate-service-account --key-file gke-stockprice-serviceaccount.json
$gcloud --quiet auth configure-docker
```

#### 操作

参考

- [イメージの管理](https://cloud.google.com/container-registry/docs/managing?hl=ja#gcloud)

imageのリスト確認

```
$gcloud container images list --repository=us.gcr.io/gke-stockprice
NAME
us.gcr.io/gke-stockprice/gke-nikkei-mock
us.gcr.io/gke-stockprice/gke-stockprice
```

imageのバージョン確認

```
$gcloud container images list-tags us.gcr.io/gke-stockprice/gke-stockprice
DIGEST        TAGS        TIMESTAMP
00a6fea8576d  545,latest  2020-09-08T05:51:03
75e258c78aba  535         2020-09-06T07:56:57
158288fd1824  528         2020-09-05T07:44:31
cad4c1d14522  524         2020-09-05T07:01:55
略
```

# GKE操作

## クラスタの作成
```
CLUSTER_NAME=gke-stockprice-cluster-prod
COMPUTE_ZONE=us-central1-f
MACHINE_TYPE=g1-small
NUM_NODES=2

gcloud --quiet container clusters create $CLUSTER_NAME \
                --machine-type=$MACHINE_TYPE --disk-size 10 --zone $COMPUTE_ZONE \
                --num-nodes=$NUM_NODES
```
- `-quiet`をつけることで作成時の「yes/no」の入力を省略できる

## クラスタの確認

```
$gcloud container clusters list
NAME                         LOCATION       MASTER_VERSION    MASTER_IP    MACHINE_TYPE  NODE_VERSION      NUM_NODES  STATUS
gke-stockprice-cluster-prod  us-central1-f  1.16.15-gke.4901  34.69.70.18  g1-small      1.16.15-gke.4901  3          RUNNING
```


## kubectlインストール

```
gcloud components update kubectl
```

- 結構時間がかかる

kubectlが使えるようになるための認証を以下のどちらかでする
方法１．Webブラウザで認証
```
$ gcloud auth login
```
- 上のコマンドを実行するとURLが表示されるので、ブラウザでURLを開くと許可を求められる
- 許可がすむと認証用の文字列が発行されるのでそれを端末に入力すればできる

方法２．service account jsonファイルで認証
```
$ gcloud auth activate-service-account --key-file gke-stockprice-serviceaccount.json
```
- GKEなどサーバ用の認証方法
- 手元にサービスアカウントのJSONファイルがあれば上の方法で認証できる

clusterに接続
```
PROJECT_NAME=gke-stockprice
CLUSTER_NAME=gke-stockprice-cluster-prod
COMPUTE_ZONE=us-central1-f
gcloud config set project $PROJECT_NAME
gcloud config set container/cluster $CLUSTER_NAME
gcloud config set compute/zone ${COMPUTE_ZONE}
gcloud container clusters get-credentials $CLUSTER_NAME
```

実行例
```
$kubectl get all
NAME                                       READY   STATUS    RESTARTS   AGE
pod/prod-gke-stockprice-1592512500-85rfl   1/2     Running   0          26h

NAME                 TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
service/kubernetes   ClusterIP   10.31.240.1   <none>        443/TCP   28h

NAME                                       COMPLETIONS   DURATION   AGE
job.batch/prod-gke-stockprice-1592512500   0/1           26h        26h

NAME                                SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
cronjob.batch/prod-gke-stockprice   */1 * * * *   False     1        26h             26h
$
```

#### ログの確認

- pod名が毎回変わるので、以下のようにpod名を取得してそのpodを指定している
- `-f` をつけることでリアルタイムで表示させる

```bash
$kubectl logs -f $(kubectl get pod | grep stockprice | awk '{print $1}') -c gke-stockprice-container
2021/01/16 07:30:42 ENV environment variable set
以下略
```

#### 実行中のコンテナへのシェルを取得

```bash
$ kubectl exec -it `kubectl get pods | grep stockprice | awk '{print $1}'` --container=gke-stockprice-container /bin/ash
/go #
```


# データの復旧手順

1. googledrive の `gke-stockprice-dump`の `stockprice-daily-XXXX.sql` をダウンロードする
2. ダウンロードしたsqlファイルをもとにデータを入れなおすには以下の２つの場合に注意する必要がある
2-1. **テーブルをゼロから作り直す場合は** そのファイルをCloudSQLに接続したMySQLクライアント上で以下のように読み込む
```
mysql> source  /mnt/c/Users/shingo/Downloads/stockprice-daily-2021-01-10.sql
```

2-2. **既存のテーブルにデータを追加したい場合は** 以下のようにファイルを修正したうえで、ファイルをCloudSQLに接続したMySQLクライアント上で以下のように読み込む

修正内容1. 以下のように`DROP`と`CREATE`をコメントアウトする
```

-- DROP TABLE IF EXISTS `daily`;
-- CREATE TABLE `daily` (
--   `code` varchar(10) NOT NULL,
--   `date` varchar(10) NOT NULL,
--   `open` varchar(15) DEFAULT NULL,
--   `high` varchar(15) DEFAULT NULL,
--   `low` varchar(15) DEFAULT NULL,
--   `close` varchar(15) DEFAULT NULL,
--   `turnover` varchar(15) DEFAULT NULL,
--   `modified` varchar(15) DEFAULT NULL,
--   PRIMARY KEY (`code`,`date`)
-- ) ENGINE=InnoDB DEFAULT CHARSET=utf8;
```

修正内容2. `INSERT`を`INSERT IGNORE` に置換する

- IGNOREをつけることで、PKがかぶったデータがすでにあればスルーするのでエラーにならない
- INSERT IGNOREだとINSERTできない場合などのエラーも無視してしまうので、より安全なのは`INSERT INTO ... ON DUPLICATE KEY UPDATE`がよさそう

修正が終わったら読み込む
```
mysql> source  /mnt/c/Users/shingo/Downloads/stockprice-daily-2021-01-10.sql
```

3. 当日の`create_gke_cluster_and_deploy_by_golang` ジョブをcircleciで実行する

- spreadsheetの **status**の当日分の実行結果を削除しておかないと、status.goの`ExecIfIncompleteThisDay`の機能で、当日分はすでに実行済みと判断されて何もされないのでspreadsheetのstatusのデータを消しておく

例：以下のような部分を削除しておく
```
saveStockPrice	1610650313	2021/01/15 3:51:53	36m26.589424776s
saveMovingAvgs	1610650603	2021/01/15 3:56:43	4m38.80397984s
calculateGrowthTrend	1610651343	2021/01/15 4:09:03	12m19.514307946s
```
