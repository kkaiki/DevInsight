## セットアップ方法

### mac
```
brew install go
```

```
go mod init dev_time_go

go get github.com/aws/aws-sdk-go
go get github.com/bwmarrin/discordgo
```

## how to make zip file
何をインポートすればいいかはAIに聞いてください

または、
依存関係の自動インストール
go mod download

または
go mod tidy

```
GOOS=linux GOARCH=amd64 go build -o bootstrap ver7.go \
&& zip function.zip bootstrap
```

* もしbootstrapという名前にしないと、lambdaが認識してくれないので注意が必要。
