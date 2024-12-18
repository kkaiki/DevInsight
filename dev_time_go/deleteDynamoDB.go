package main

import (
    "context"
    "fmt"
    "log"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
    tableName = "dev_insight"
    batchSize = 25
    maxWorkers = 5
)

func handleRequest(ctx context.Context) error {
    log.Println("Lambda関数が呼び出されました")
    return deleteAllItems()
}

func deleteAllItems() error {
    log.Println("関数の実行を開始します")

    sess := session.Must(session.NewSession(&aws.Config{
        Region: aws.String("ap-northeast-1"),
    }))
    svc := dynamodb.New(sess)
    log.Println("DynamoDB クライアントを初期化しました")

    var lastKey map[string]*dynamodb.AttributeValue
    totalDeleted := 0

    for {
        scanInput := &dynamodb.ScanInput{
            TableName: aws.String(tableName),
            Limit:    aws.Int64(batchSize),
            ExclusiveStartKey: lastKey,
        }

        result, err := svc.Scan(scanInput)
        if err != nil {
            log.Printf("スキャンエラー: %v", err)
            return fmt.Errorf("スキャンエラー: %v", err)
        }

        if len(result.Items) == 0 {
            break
        }

        var writeRequests []*dynamodb.WriteRequest
        for _, item := range result.Items {
            writeRequests = append(writeRequests, &dynamodb.WriteRequest{
                DeleteRequest: &dynamodb.DeleteRequest{
                    Key: map[string]*dynamodb.AttributeValue{
                        "discord_id": item["discord_id"],
                        "timestamp":  item["timestamp"],
                    },
                },
            })
        }

        if len(writeRequests) > 0 {
            input := &dynamodb.BatchWriteItemInput{
                RequestItems: map[string][]*dynamodb.WriteRequest{
                    tableName: writeRequests,
                },
            }

            _, err = svc.BatchWriteItem(input)
            if err != nil {
                log.Printf("バッチ削除エラー: %v", err)
                return fmt.Errorf("バッチ削除エラー: %v", err)
            }
            totalDeleted += len(writeRequests)
            log.Printf("%d件のアイテムを削除しました", len(writeRequests))
        }

        lastKey = result.LastEvaluatedKey
        if lastKey == nil {
            break
        }
    }

    log.Printf("合計%d件のアイテムを削除しました", totalDeleted)
    return nil
}

func main() {
    log.Println("main関数を開始します")
    lambda.Start(handleRequest)
}
