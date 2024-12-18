package main

import (
    "context"
    "fmt"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
)

const tableName = "dev_insight"

func deleteAllItems() error {
    sess := session.Must(session.NewSession(&aws.Config{
        Region: aws.String("ap-northeast-1"),
    }))
    svc := dynamodb.New(sess)

    // 全アイテムをスキャン
    scanInput := &dynamodb.ScanInput{
        TableName: aws.String(tableName),
    }
    
    result, err := svc.Scan(scanInput)
    if err != nil {
        return fmt.Errorf("スキャンエラー: %v", err)
    }

    if len(result.Items) == 0 {
        fmt.Println("テーブルは空です")
        return nil
    }

    // 各アイテムを削除
    for _, item := range result.Items {
        deleteInput := &dynamodb.DeleteItemInput{
            TableName: aws.String(tableName),
            Key: map[string]*dynamodb.AttributeValue{
                "discord_id": item["discord_id"],
                "timestamp":  item["timestamp"],
            },
        }

        if _, err := svc.DeleteItem(deleteInput); err != nil {
            return fmt.Errorf("削除エラー: %v", err)
        }
    }

    fmt.Printf("%d件のアイテムを削除しました\n", len(result.Items))
    return nil
}

func handleRequest(ctx context.Context) error {
    return deleteAllItems()
}

func main() {
    lambda.Start(handleRequest)
}