package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
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
	Language  string `json:"language"`
}

type DiscordWorkTime struct {
	DiscordID       string
	DiscordUniqueID string
	TotalTime       time.Duration
	Languages       map[string]time.Duration
}

func validateEnv() error {
	log.Printf("[DEBUG] validateEnv called")
	discordToken := os.Getenv("DISCORD_TOKEN")
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	otherLanguages := os.Getenv("OTHER_LANGUAGES")
	mergeLanguages := os.Getenv("MERGE_LANGUAGES")

	log.Printf("[DEBUG] DISCORD_TOKEN: %v", len(discordToken) > 0)
	log.Printf("[DEBUG] DISCORD_CHANNEL_ID: %v", channelID)
	log.Printf("[DEBUG] OTHER_LANGUAGES: %v", otherLanguages)
	log.Printf("[DEBUG] MERGE_LANGUAGES: %v", mergeLanguages)

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
	if otherLanguages == "" {
		return &AppError{
			Type:    "ConfigError",
			Message: "OTHER_LANGUAGES が設定されていません",
		}
	}
	if mergeLanguages == "" {
		return &AppError{
			Type:    "ConfigError",
			Message: "MERGE_LANGUAGES が設定されていません",
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

func formatMessage(dg *discordgo.Session, data []DiscordWorkTime) string {
	log.Printf("[DEBUG] formatMessage called, data len: %d", len(data))
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
		log.Printf("[DEBUG] Ranking %d: DiscordID=%s, TotalTime=%v", i+1, entry.DiscordID, entry.TotalTime)
		// 1時間未満の場合はスキップ
		if entry.TotalTime < time.Hour {
			log.Printf("[DEBUG] Skipping DiscordID=%s, TotalTime=%v (less than 1 hour)", entry.DiscordID, entry.TotalTime)
			continue
		}

		var rankPrefix string
		switch i {
		case 0:
			rankPrefix = "# 🥇 "
		case 1:
			rankPrefix = "## 🥈 "
		case 2:
			rankPrefix = "### 🥉 "
		default:
			rankPrefix = ""
		}

		displayName := fmt.Sprintf("<@%s>", entry.DiscordUniqueID)

		hours := int(entry.TotalTime.Hours())
		minutes := int(entry.TotalTime.Minutes()) % 60

		message += fmt.Sprintf("%s%s %d時間%d分\n",
			rankPrefix,
			displayName,
			hours,
			minutes,
		)

		// トップ3の言語とその使用時間を追加
		sortedLanguages := sortLanguagesByTime(entry.Languages)
		for j, lang := range sortedLanguages {
			if j >= 3 {
				break
			}
			log.Printf("[DEBUG]   Language Rank %d: %s %v", j+1, lang.Name, lang.Time)
			langHours := int(lang.Time.Hours())
			langMinutes := int(lang.Time.Minutes()) % 60
			message += fmt.Sprintf("  - %s: %d時間%d分\n", lang.Name, langHours, langMinutes)
		}
	}

	message += "========================\n"
	message += "[\n\nダウンロード](https://marketplace.visualstudio.com/items?itemName=DevInsights.vscode-DevInsights)\n"
	return message
}

func handleRequest(ctx context.Context) error {
	log.Printf("[DEBUG] handleRequest called")
	if err := validateEnv(); err != nil {
		logError(err)
		return err
	}

	discordToken := os.Getenv("DISCORD_TOKEN")
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	// トークンの先頭・末尾をマスクして出力
	maskedToken := ""
	if len(discordToken) > 8 {
		maskedToken = discordToken[:4] + "..." + discordToken[len(discordToken)-4:]
	} else {
		maskedToken = "(short or empty)"
	}
	log.Printf("[DEBUG] Creating Discord session. Token(partial): %s, ChannelID: %s", maskedToken, channelID)
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Printf("[ERROR] discordgo.New failed: %+v", err)
		logError(err)
		return &AppError{
			Type:    "DiscordError",
			Message: "Discordセッションの作成に失敗",
			Err:     err,
		}
	}
	log.Printf("[DEBUG] Discord session created: %+v", dg)

	err = dg.Open()
	if err != nil {
		log.Printf("[ERROR] dg.Open failed: %+v", err)
		// Discord APIのレスポンスやエラー詳細を出力
		log.Printf("[DEBUG] Discord session state: %+v", dg.State)
		logError(err)
		return &AppError{
			Type:    "DiscordError",
			Message: "Discordセッションのオープンに失敗",
			Err:     err,
		}
	}
	log.Printf("[DEBUG] Discord session opened successfully.")
	defer dg.Close()

	// チャンネル情報取得で権限や存在確認
	ch, chErr := dg.State.Channel(channelID)
	if chErr != nil || ch == nil {
		log.Printf("[ERROR] Channel not found in state: %v", chErr)
		// APIからも取得を試みる
		ch, chErr = dg.Channel(channelID)
		if chErr != nil {
			log.Printf("[ERROR] Channel fetch from API failed: %v", chErr)
		} else {
			log.Printf("[DEBUG] Channel fetched from API: %+v", ch)
		}
	} else {
		log.Printf("[DEBUG] Channel found in state: %+v", ch)
	}

	log.Printf("[DEBUG] Getting sorted Discord data")
	sortedData := getSortedDiscordData()
	if sortedData == nil {
		logError(&AppError{Type: "DataError", Message: "データの取得に失敗"})
		return &AppError{
			Type:    "DataError",
			Message: "データの取得に失敗",
		}
	}

	log.Printf("[DEBUG] Formatting message for Discord")
	message := formatMessage(dg, sortedData)
	log.Printf("[DEBUG] Sending message to Discord channel: %s", channelID)
	if err := sendDiscordMessage(dg, channelID, message); err != nil {
		logError(err)
		return err
	}

	log.Printf("[DEBUG] handleRequest completed successfully")
	return nil
}

type LanguageTime struct {
	Name string
	Time time.Duration
}

func sortLanguagesByTime(languages map[string]time.Duration) []LanguageTime {
	var langTimes []LanguageTime
	for name, time := range languages {
		langTimes = append(langTimes, LanguageTime{Name: name, Time: time})
	}
	sort.Slice(langTimes, func(i, j int) bool {
		return langTimes[i].Time > langTimes[j].Time
	})
	return langTimes
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

// 言語のマッピングを取得
func getLanguageMapping() map[string]string {
	log.Printf("[DEBUG] getLanguageMapping called")
	mergeLanguages := os.Getenv("MERGE_LANGUAGES")
	log.Printf("[DEBUG] MERGE_LANGUAGES: %v", mergeLanguages)
	mapping := make(map[string]string)
	pairs := strings.Split(mergeLanguages, ",")
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) == 2 {
			mapping[kv[0]] = kv[1]
		}
	}
	return mapping
}

func getDiscordIDAndTimes(discordID string) ([]time.Time, []string, error) {
	log.Printf("[DEBUG] getDiscordIDAndTimes called for DiscordID=%s", discordID)
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

	log.Printf("[DEBUG] データ取得期間: %s - %s", startDate.Format(time.RFC3339), now.Format(time.RFC3339))

	// フィルター条件を組み立て
	keyCond := expression.Key("discord_id").Equal(expression.Value(discordID)).
		And(expression.Key("timestamp").GreaterThanEqual(expression.Value(startDate.Format(time.RFC3339))))

	// 複合条件を作成
	builder := expression.NewBuilder().WithKeyCondition(keyCond)
	expr, err := builder.Build()
	if err != nil {
		return nil, nil, &AppError{
			Type:    "DynamoDBError",
			Message: "クエリ式の構築に失敗",
			Err:     err,
		}
	}

	// DynamoDBクエリを実行
	result, err := svc.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})

	if err != nil {
		logError(err)
		return nil, nil, &AppError{
			Type:    "DynamoDBError",
			Message: "クエリの実行に失敗",
			Err:     err,
		}
	}

	log.Printf("[DEBUG] DynamoDB Query returned %d items", len(result.Items))

	var items []InsightData
	if err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &items); err != nil {
		logError(err)
		return nil, nil, &AppError{
			Type:    "DataError",
			Message: "データのアンマーシャルに失敗",
			Err:     err,
		}
	}

	log.Printf("[DEBUG] Unmarshaled %d InsightData items", len(items))

	var times []time.Time
	var languages []string
	languageMapping := getLanguageMapping() // 言語のマッピングを取得
	for _, item := range items {
		t, err := time.Parse(time.RFC3339, item.Timestamp)
		if err != nil {
			log.Printf("[エラー] タイムスタンプの解析に失敗: %v", err)
			continue
		}
		times = append(times, t)
		language := item.Language
		if mappedLanguage, ok := languageMapping[language]; ok {
			log.Printf("[DEBUG] Language %s mapped to %s", language, mappedLanguage)
			language = mappedLanguage // 言語のマッピングを適用
		}
		languages = append(languages, language)
	}

	log.Printf("[DEBUG] Returning %d times and %d languages", len(times), len(languages))

	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	return times, languages, nil
}

func getUniqueDiscordIDs() (map[string]string, error) {
	log.Printf("[DEBUG] getUniqueDiscordIDs called")
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
		logError(err)
		return nil, &AppError{
			Type:    "DynamoDBError",
			Message: "DynamoDBのスキャンに失敗",
			Err:     err,
		}
	}

	log.Printf("[DEBUG] DynamoDB Scan returned %d items", len(result.Items))

	// 結果をアンマーシャル
	var items []InsightData
	uniqueDiscordIDs := make(map[string]string)

	if err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &items); err != nil {
		logError(err)
		return nil, &AppError{
			Type:    "DataError",
			Message: "データのアンマーシャルに失敗",
			Err:     err,
		}
	}

	log.Printf("[DEBUG] Unmarshaled %d InsightData items", len(items))

	// ユニークなDiscord IDを収集
	for _, item := range items {
		uniqueDiscordIDs[item.DiscordID] = item.DiscordID // DiscordUniqueIDとして設定
	}
	log.Printf("[DEBUG] Collected %d unique Discord IDs", len(uniqueDiscordIDs))

	return uniqueDiscordIDs, nil
}

func getSortedDiscordData() []DiscordWorkTime {
	log.Printf("[DEBUG] getSortedDiscordData called")
	// 1. Discord IDの取得
	discordIDMap, err := getUniqueDiscordIDs()
	if err != nil {
		log.Printf("[エラー] Discord IDの取得に失敗: %v", err)
		return nil
	}
	log.Printf("[情報] 取得したDiscord ID数: %d", len(discordIDMap))

	if len(discordIDMap) == 0 {
		log.Printf("[警告] 対象期間内のデータが見つかりません")
		return nil
	}

	// 2. 各ユーザーの言語ごとの時間データ取得
	var data []DiscordWorkTime
	for discordID, discordUniqueID := range discordIDMap {
		log.Printf("[DEBUG] Processing DiscordID=%s", discordID)
		times, languages, err := getDiscordIDAndTimes(discordID)
		if err != nil {
			log.Printf("[エラー] 言語データの取得失敗 (ID: %s): %v", discordID, err)
			continue
		}
		log.Printf("[情報] ユーザー %s の言語データ数: %d", discordID, len(languages))

		if len(times) > 0 {
			sessionTimes, languageDurations := calculateSessionTimes(times, languages)
			totalWorkTime := getTotalWorkTime(sessionTimes)
			log.Printf("[DEBUG] User %s: totalWorkTime=%v, sessionCount=%d", discordID, totalWorkTime, len(sessionTimes))
			data = append(data, DiscordWorkTime{
				DiscordID:       discordID,
				DiscordUniqueID: discordUniqueID,
				TotalTime:       totalWorkTime,
				Languages:       languageDurations,
			})
			log.Printf("[情報] ユーザー %s の合計作業時間: %v", discordID, totalWorkTime)
		}
	}

	if len(data) == 0 {
		log.Printf("[警告] 集計可能なデータが見つかりません")
		return nil
	}

	// 作業時間でソート
	log.Printf("[DEBUG] Sorting DiscordWorkTime by TotalTime")
	sort.Slice(data, func(i, j int) bool {
		return data[i].TotalTime > data[j].TotalTime
	})

	return data
}

func getTotalWorkTime(sessionTimes []SessionTime) time.Duration {
	var totalTime time.Duration
	for _, session := range sessionTimes {
		totalTime += session.End.Sub(session.Start)
	}
	return totalTime
}

type SessionTime struct {
	Start    time.Time
	End      time.Time
	Language string
}

func calculateSessionTimes(times []time.Time, languages []string) ([]SessionTime, map[string]time.Duration) {
	log.Printf("[DEBUG] calculateSessionTimes called, times len: %d, languages len: %d", len(times), len(languages))
	var sessionTimes []SessionTime
	languageDurations := make(map[string]time.Duration)

	if len(times) == 0 {
		return sessionTimes, languageDurations
	}

	sessionStart := times[0]
	sessionEnd := times[0]
	currentLanguage := languages[0]

	for i := 1; i < len(times); i++ {
		if times[i].Sub(sessionEnd) > 5*time.Minute {
			log.Printf("[DEBUG] New session detected at i=%d, prevEnd=%v, newStart=%v", i, sessionEnd, times[i])
			sessionTimes = append(sessionTimes, SessionTime{
				Start:    sessionStart,
				End:      sessionEnd,
				Language: currentLanguage,
			})
			languageDurations[currentLanguage] += sessionEnd.Sub(sessionStart)
			sessionStart = times[i]
			currentLanguage = languages[i]
		}
		sessionEnd = times[i]
	}

	sessionTimes = append(sessionTimes, SessionTime{
		Start:    sessionStart,
		End:      sessionEnd,
		Language: currentLanguage,
	})
	languageDurations[currentLanguage] += sessionEnd.Sub(sessionStart)

	log.Printf("[DEBUG] Returning %d sessionTimes, %d languageDurations", len(sessionTimes), len(languageDurations))
	return sessionTimes, languageDurations
}

func main() {
	lambda.Start(handleRequest)
}
