# DevInsights for Visual Studio Code

[DevInsights] は、Visual Studio Code でのコーディングに費やした時間を自動的に追跡し、生産性に関する詳細な洞察を提供するツールです。また、Discord と統合されており、総コーディング時間をランキング表示することで、どのユーザーが最も作業したかを確認できます。さらに、DevInsights は複数の IDE やエディタと連携できるため、さまざまなプラットフォームでのコーディング活動を包括的に把握できます。

## インストール

1. F1 キーまたは ⌘ + Shift + P を押して「install」と入力します。次に「Extensions: Install Extension」を選択します。
![コマンドパレットを開く](images/extentions_install.png)

2. 「devinsights」と入力し、Enter キーを押します。

3. Discord ID を入力し、Enter キーを押します。
   （プロンプトが表示されない場合は、F1 キーまたは ⌘ + Shift + P を押して「DevInsights Discord ID」と入力してください。）

   **Discord ID のコピー方法:**
   - Discord を開き、ユーザー設定に移動します。
   ![コマンドパレットを開く](images/user_setting.png)
   - 「詳細設定」の下で「開発者モード」を有効にします。
   ![コマンドパレットを開く](images/developer_mode.png)
   - ユーザーリスト内のユーザー名またはプロフィールを右クリックし、「ID をコピー」を選択します。
   ![コマンドパレットを開く](images/copy_userid.png)

4. VS Code を使用すると、コーディング活動が毎週ランキングされます。

## 使用方法

毎週、Discord ボットがユーザーをコーディングに費やした時間に基づいてランク付けし、コーディング活動のリーダーボードを提供します。

## 設定

**VS Code の設定:**
⌘ + Shift + P を押して「DevInsights: Input Discord Id」と入力し、Discord ID を設定します。

## トラブルシューティング

**エラーログの確認:**
DevInsights の範囲外のエラーは、`$HOME/.devinsights/devinsights.log` ファイルに記録されます。

**Windows ユーザー向け情報:**
DevInsights を企業のプロキシの背後で使用している場合、[win-ca] 拡張機能をインストールして VS Code 内で Windows のルート証明書を有効にすることをお勧めします。Ctrl + Shift + X を押して「win-ca」を検索し、インストールします。

### SSH 設定

[ssh extension](https://code.visualstudio.com/docs/remote/ssh) を使用してリモートホストに接続している場合、DevInsights をサーバーではなくローカルで実行するよう強制することを検討してください。この設定は、接続先のサーバーが他の人と共有されている場合に必要です。 [こちら](https://code.visualstudio.com/docs/remote/ssh#_advanced-forcing-an-extension-to-run-locally-remotely) のガイドに従ってください。

## アンインストール

1. VS Code の Extensions サイドバー項目をクリックします。

2. 「devinsights」と入力し、Enter キーを押します。

3. DevInsights の横にある設定アイコンをクリックし、「Uninstall」を選択します。

4. ホームディレクトリ内の `~/.devinsights*` ファイルを削除します。ただし、他の IDE で DevInsights をまだ使用している場合は削除しないでください。
