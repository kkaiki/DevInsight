package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "sort"
    "time"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/aws/aws-sdk-go/service/dynamodb/expression"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/bwmarrin/discordgo"
)

// カスタムエラー型
type AppError struct {
    Type    string
    Message string
    Err     error
}

func (e *AppError) Error() string {
    return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
}

var (
    svc = dynamodb.New(session.Must(session.NewSession(&aws.Config{
        Region: aws.String("ap-northeast-1"),
    })))
    tableName = "dev_insight"
)

type InsightData struct {
    DiscordID string `json:"discord_id"`
    Timestamp string `json:"timestamp"`
}

type DiscordWorkTime struct {
    DiscordID string
    TotalTime time.Duration
}

// 環境変数の検証
func validateEnv() error {
    discordToken := os.Getenv("DISCORD_TOKEN")
    channelID := os.Getenv("DISCORD_CHANNEL_ID")
    
    if discordToken == "" {
        return &AppError{
            Type:    "ConfigError",
            Message: "DISCORD_TOKEN が設定されていません",
        }
    }
    if channelID == "" {
        return &AppError{
            Type:    "ConfigError",
            Message: "DISCORD_CHANNEL_ID が設定されていません",
        }
    }
    return nil
}


// エラーログの強化
func logError(err error) {
    if appErr, ok := err.(*AppError); ok {
        log.Printf("[%s] %s: %v", appErr.Type, appErr.Message, appErr.Err)
    } else {
        log.Printf("[UnknownError] %v", err)
    }
}

// handleRequestにエラーログを追加
func handleRequest(ctx context.Context) error {
    if err := validateEnv(); err != nil {
        logError(err)
        return err
    }

    discordToken := os.Getenv("DISCORD_TOKEN")
    channelID := os.Getenv("DISCORD_CHANNEL_ID")

    dg, err := discordgo.New("Bot " + discordToken)
    if err != nil {
        return &AppError{
            Type:    "DiscordError",
            Message: "Discordセッションの作成に失敗",
            Err:     err,
        }
    }

    err = dg.Open()
    if err != nil {
        return &AppError{
            Type:    "DiscordError",
            Message: "Discordセッションのオープンに失敗",
            Err:     err,
        }
    }
    defer dg.Close()

    sortedData := getSortedDiscordData()
    if sortedData == nil {
        return &AppError{
            Type:    "DataError",
            Message: "データの取得に失敗",
        }
    }

    message := formatMessage(sortedData)
    if err := sendDiscordMessage(dg, channelID, message); err != nil {
        return err
    }

    return nil
}

func formatMessage(data []DiscordWorkTime) string {
    if len(data) == 0 {
        return "データがありません。"
    }
    
    now := time.Now().UTC()
    sevenDaysAgo := now.AddDate(0, 0, -7)
    startDate := time.Date(
        sevenDaysAgo.Year(),
        sevenDaysAgo.Month(),
        sevenDaysAgo.Day(),
        0, 0, 0, 0,
        sevenDaysAgo.Location(),
    )
    
    message := fmt.Sprintf("作業時間ランキング (%s から)\n", startDate.Format("2006/01/02"))
    message += "========================\n"
    
    for i, entry := range data {
        hours := int(entry.TotalTime.Hours())
        minutes := int(entry.TotalTime.Minutes()) % 60
        
        message += fmt.Sprintf("%d位: <@%s> %d時間%d分\n",
            i+1,
            entry.DiscordID,
            hours,
            minutes,
        )
    }
    
    message += "========================\n"
    return message
}

func sendDiscordMessage(dg *discordgo.Session, channelID, message string) error {
    _, err := dg.ChannelMessageSend(channelID, message)
    if err != nil {
        return &AppError{
            Type:    "DiscordError",
            Message: "メッセージの送信に失敗",
            Err:     err,
        }
    }
    return nil
}

func getDiscordIDAndTimes(discordID string) ([]time.Time, error) {
    // 7日前の日付を計算
    now := time.Now().UTC()
    sevenDaysAgo := now.AddDate(0, 0, -7)
    startDate := time.Date(
        sevenDaysAgo.Year(),
        sevenDaysAgo.Month(),
        sevenDaysAgo.Day(),
        0, 0, 0, 0,
        sevenDaysAgo.Location(),
    )

    // フィルター条件を組み立て
    keyCond := expression.Key("discord_id").Equal(expression.Value(discordID))
    timeCond := expression.Name("timestamp").GreaterThanEqual(expression.Value(startDate.Format(time.RFC3339)))
    
    // 複合条件を作成
    builder := expression.NewBuilder().WithKeyCondition(keyCond).WithFilter(timeCond)
    expr, err := builder.Build()
    if err != nil {
        return nil, &AppError{
            Type:    "DynamoDBError",
            Message: "クエリ式の構築に失敗",
            Err:     err,
        }
    }

    // DynamoDBクエリを実行
    result, err := svc.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(tableName),
        KeyConditionExpression:    expr.KeyCondition(),
        FilterExpression:         expr.Filter(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    })

    var items []InsightData
    if err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &items); err != nil {
        return nil, &AppError{
            Type:    "DataError",
            Message: "データのアンマーシャルに失敗",
            Err:     err,
        }
    }

    var times []time.Time
    for _, item := range items {
        t, err := time.Parse(time.RFC3339, item.Timestamp)
        if err != nil {
            log.Printf("タイムスタンプの解析に失敗: %v", err)
            continue
        }
        times = append(times, t)
    }

    sort.Slice(times, func(i, j int) bool {
        return times[i].Before(times[j])
    })

    return times, nil
}

func getUniqueDiscordIDs() ([]string, error) {
    // 現在時刻（UTC）
    now := time.Now().UTC()
    
    // 7日前の0時を計算
    sevenDaysAgo := now.AddDate(0, 0, -7)
    startDate := time.Date(
        sevenDaysAgo.Year(),
        sevenDaysAgo.Month(),
        sevenDaysAgo.Day(),
        0, 0, 0, 0,
        sevenDaysAgo.Location(),
    )

    // DynamoDBのフィルター式を構築
    filt := expression.Name("timestamp").GreaterThanEqual(expression.Value(startDate.Format(time.RFC3339)))
    expr, err := expression.NewBuilder().WithFilter(filt).Build()
    if err != nil {
        return nil, &AppError{
            Type:    "DynamoDBError",
            Message: "クエリ式の構築に失敗",
            Err:     err,
        }
    }

    // DynamoDBをスキャン
    result, err := svc.Scan(&dynamodb.ScanInput{
        TableName:                 aws.String(tableName),
        FilterExpression:          expr.Filter(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    })
    if err != nil {
        return nil, &AppError{
            Type:    "DynamoDBError",
            Message: "DynamoDBのスキャンに失敗",
            Err:     err,
        }
    }

    // 結果をアンマーシャル
    var items []InsightData
    uniqueDiscordIDs := map[string]bool{}
    
    if err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &items); err != nil {
        return nil, &AppError{
            Type:    "DataError",
            Message: "データのアンマーシャルに失敗",
            Err:     err,
        }
    }

    // ユニークなDiscord IDを収集
    for _, item := range items {
        uniqueDiscordIDs[item.DiscordID] = true
    }

    var discordIDs []string
    for id := range uniqueDiscordIDs {
        discordIDs = append(discordIDs, id)
    }

    return discordIDs, nil
}


func getSortedDiscordData() []DiscordWorkTime {
    // 1. Discord IDの取得
    discordIDs, err := getUniqueDiscordIDs()
    if err != nil {
        log.Printf("[エラー] Discord IDの取得に失敗: %v", err)
        return nil
    }
    log.Printf("[情報] 取得したDiscord ID数: %d", len(discordIDs))

    if len(discordIDs) == 0 {
        log.Printf("[警告] 対象期間内のデータが見つかりません")
        return nil
    }

    // 2. 各ユーザーの時間データ取得
    var data []DiscordWorkTime
    for _, discordID := range discordIDs {
        times, err := getDiscordIDAndTimes(discordID)
        if err != nil {
            log.Printf("[エラー] 時間データの取得失敗 (ID: %s): %v", discordID, err)
            continue
        }
        log.Printf("[情報] ユーザー %s の時間データ数: %d", discordID, len(times))

        if len(times) > 0 {
            sessionTimes := calculateSessionTimes(times)
            totalWorkTime := getTotalWorkTime(sessionTimes)
            data = append(data, DiscordWorkTime{
                DiscordID: discordID,
                TotalTime: totalWorkTime,
            })
            log.Printf("[情報] ユーザー %s の合計作業時間: %v", discordID, totalWorkTime)
        }
    }

    if len(data) == 0 {
        log.Printf("[警告] 集計可能なデータが見つかりません")
        return nil
    }

    return data
}

func calculateSessionTimes(times []time.Time) []struct {
	Start time.Time
	End   time.Time
} {
	var sessionTimes []struct {
		Start time.Time
		End   time.Time
	}

	sessionStart := times[0]
	sessionEnd := times[0]

	for i := 1; i < len(times); i++ {
		if times[i].Sub(sessionEnd) > 5*time.Minute {
			sessionTimes = append(sessionTimes, struct {
				Start time.Time
				End   time.Time
			}{Start: sessionStart, End: sessionEnd})
			sessionStart = times[i]
		}
		sessionEnd = times[i]
	}

	sessionTimes = append(sessionTimes, struct {
		Start time.Time
		End   time.Time
	}{Start: sessionStart, End: sessionEnd})

	return sessionTimes
}

// セッションタイムの総合時間を計算する関数
func getTotalWorkTime(sessionTimes []struct {
	Start time.Time
	End   time.Time
}) time.Duration {
	var totalTime time.Duration
	for _, session := range sessionTimes {
		totalTime += session.End.Sub(session.Start)
	}
	return totalTime
}

func main() {
    lambda.Start(handleRequest)
}
