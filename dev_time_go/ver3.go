package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/bwmarrin/discordgo"
)

// DynamoDBのクライアントを初期化
var sess = session.Must(session.NewSession(&aws.Config{
	Region: aws.String("ap-northeast-1"),
}))

var svc = dynamodb.New(sess)
var tableName = "dev_insight"

// データの構造体
type InsightData struct {
	DiscordID string `json:"discord_id"`
	Timestamp string `json:"timestamp"`
}

// Discord IDと総作業時間を保持する構造体
type DiscordWorkTime struct {
	DiscordID string
	TotalTime time.Duration
}

// DiscordボットのトークンとチャンネルID
var discordToken = "MTI4MjAwNjc1MDQ5MjM2NDk4MA.GJqJcq.t0YORbdSk_gYrgnZ5ddKSEgYn44mAmYTk5ciEE"
var channelID = "1276135174853099622"

func main() {
	// Discordセッションを作成
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %s", err)
	}

	// ボットを起動
	err = dg.Open()
	if err != nil {
		log.Fatalf("Failed to open Discord session: %s", err)
	}
	defer dg.Close()

	sortedData := getSortedDiscordData()
	if sortedData != nil {
		message := "作業時間ランキング:\n"
		for _, entry := range sortedData {
			message += fmt.Sprintf("Discord ID: %s, Total Work Time: %s\n", entry.DiscordID, entry.TotalTime)
		}
		// メッセージを送信
		_, err := dg.ChannelMessageSend(channelID, message)
		if err != nil {
			log.Fatalf("Failed to send message: %s", err)
		}
	} else {
		_, err := dg.ChannelMessageSend(channelID, "データがありません。")
		if err != nil {
			log.Fatalf("Failed to send message: %s", err)
		}
	}
}

// ユニークなDiscord IDを取得する関数
func getUniqueDiscordIDs() ([]string, error) {
	now := time.Now().UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7)

	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Fatalf("Failed to scan items: %s", err)
		return nil, err
	}

	var items []InsightData
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		log.Fatalf("Failed to unmarshal items: %s", err)
		return nil, err
	}

	uniqueDiscordIDs := map[string]bool{}
	for _, item := range items {
		timestamp, _ := time.Parse(time.RFC3339, item.Timestamp)
		if timestamp.After(sevenDaysAgo) {
			uniqueDiscordIDs[item.DiscordID] = true
		}
	}

	var discordIDs []string
	for id := range uniqueDiscordIDs {
		discordIDs = append(discordIDs, id)
	}

	return discordIDs, nil
}

// Discord IDに関連する全てのタイムスタンプを取得する関数
func getDiscordIDAndTimes(discordID string) ([]time.Time, error) {
	filt := expression.Key("discord_id").Equal(expression.Value(discordID))
	expr, err := expression.NewBuilder().WithKeyCondition(filt).Build()
	if err != nil {
		log.Fatalf("Failed to build expression: %s", err)
		return nil, err
	}

	result, err := svc.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		log.Fatalf("Failed to query items: %s", err)
		return nil, err
	}

	var items []InsightData
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		log.Fatalf("Failed to unmarshal items: %s", err)
		return nil, err
	}

	var times []time.Time
	for _, item := range items {
		t, err := time.Parse(time.RFC3339, item.Timestamp)
		if err != nil {
			log.Fatalf("Failed to parse timestamp: %s", err)
		}
		times = append(times, t)
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	return times, nil
}

// セッションタイムを計算する関数
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

// ソートされたDiscordデータを取得する関数
func getSortedDiscordData() []DiscordWorkTime {
	discordIDs, err := getUniqueDiscordIDs()
	if err != nil {
		log.Fatalf("Failed to get unique Discord IDs: %s", err)
		return nil
	}

	var data []DiscordWorkTime

	for _, discordID := range discordIDs {
		times, err := getDiscordIDAndTimes(discordID)
		if err != nil {
			log.Printf("Failed to get times for Discord ID: %s", discordID)
			continue
		}

		if len(times) > 0 {
			sessionTimes := calculateSessionTimes(times)
			totalWorkTime := getTotalWorkTime(sessionTimes)
			data = append(data, DiscordWorkTime{
				DiscordID: discordID,
				TotalTime: totalWorkTime,
			})
		}
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].TotalTime > data[j].TotalTime
	})

	return data
}
