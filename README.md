# DevInsights

## Backgroud and Summary

[DevInsights] は、コーディングに費やした時間を自動的に追跡し可視化するツールです。また、Discordと統合されており、総コーディング時間をランキング表示することで、どのユーザーが最も作業したかを確認できます！新たな視点からどれくらいメンバーが頑張っているかを確認できます！

## インストール

1. 拡張機能ストアへ移動してください！
![コマンドパレットを開く](https://kkaiki.github.io/DevInsight/images/extentions_install.png)

2. 「devinsights」と入力し、Enter キーを押します。

3. Discord Unique ID を入力し、Enter キーを押します。
   （入力欄が表示されない場合は、F1 キーまたは ⌘ + Shift + P を押して「DevInsights Discord ID」などと入力してください。）

   **Discord Unique ID のコピー方法:**
   - Discord を開き、ユーザー設定に移動します。
   ![コマンドパレットを開く](https://kkaiki.github.io/DevInsight/images/user_setting.png)
   - 「詳細設定」の下で「開発者モード」を有効にします。
   ![コマンドパレットを開く](https://kkaiki.github.io/DevInsight/images/developer_mode.png)
   - ユーザーリスト内のユーザー名またはプロフィールを右クリックし、「ID をコピー」を選択します。
   ![コマンドパレットを開く](https://kkaiki.github.io/DevInsight/images/copy_userid.png)

4. VS Code を使用すると、コーディング活動が毎週ランキングされます。

## 使用方法

毎週、Discord ボットがユーザーをコーディングに費やした時間に基づいてランク付けし、コーディング活動のリーダーボードを提供します。

使用時間はコーディングをしている時間またはファイルを切り替えた時がカウントされるため、ただエディタを開いているだけでは正しく計測されません。
