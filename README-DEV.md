## セットアップ方法

```sh
rm -rf node_modules
rm -rf dist
npm cache clean --force
npm install
npm run vscode:prepublish
```


## 拡張機能化
1. npmを使用してvscodeの拡張機能化するためのものをインストール
```
sudo npm install -g vsce
```
もし上記の方法で解決しない場合は、`--force`オプションを使用してインストールを強制する

2. 拡張機能化
```
vsce package
```


## tips

### 強制的にバージョンアップを行う方法

package.jsonファイルを変更してもバージョンアップされないエラーに遭遇したため記載(m1 macbook air)
npm version 26.0.1 --force --no-git-tag-version


