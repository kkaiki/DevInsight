package main

import (
    "fmt"
    "log"
    "sort"
    "time"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/aws/aws-sdk-go/service/dynamodb/expression"
    "github.com/bwmarrin/discordgo"
)

// Initialize AWS and Discord sessions
var sess = session.Must(session.NewSession(&aws.Config{
    Region: aws.String("ap-northeast-1"),
}))
var svc = dynamodb.New(sess)
var tableName = "dev_insight"

// Discord bot token and guild ID
const (
    Token   = "MTI4MjAwNjc1MDQ5MjM2NDk4MA.GJqJcq.t0YORbdSk_gYrgnZ5ddKSEgYn44mAmYTk5ciEE"
    GuildID = "1052167237106159676"
)

// Data structure
type InsightData struct {
    DiscordID string `json:"discord_id"`
    Timestamp string `json:"timestamp"`
    Language  string `json:"language"`
}

type DiscordWorkTime struct {
    DiscordID    string
    TotalTime    time.Duration
    LanguageTimes map[string]time.Duration
}

func main() {
    lambda.Start(handler)
}

func handler() {
    sortedData := getSortedDiscordData()
    if sortedData != nil {
        err := assignRoles(sortedData)
        if err != nil {
            log.Fatalf("Failed to assign roles: %s", err)
        }
    }
}

func getSortedDiscordData() []DiscordWorkTime {
    discordIDs, err := getUniqueDiscordIDs()
    if err != nil {
        log.Fatalf("Failed to get unique Discord IDs: %s", err)
        return nil
    }

    var data []DiscordWorkTime
    for _, discordID := range discordIDs {
        items, err := getDiscordIDAndTimes(discordID)
        if err != nil {
            log.Printf("Failed to get data for Discord ID: %s", discordID)
            continue
        }

        if len(items) > 0 {
            times := extractTimesFromItems(items)
            sessionTimes := calculateSessionTimes(times)
            totalWorkTime := getTotalWorkTime(sessionTimes)
            languageTimes := calculateLanguageTimes(items)
            data = append(data, DiscordWorkTime{
                DiscordID:    discordID,
                TotalTime:    totalWorkTime,
                LanguageTimes: languageTimes,
            })
        }
    }

    sort.Slice(data, func(i, j int) bool {
        return data[i].TotalTime > data[j].TotalTime
    })

    return data
}

func getUniqueDiscordIDs() ([]string, error) {
    now := time.Now().UTC()
    sevenDaysAgo := now.AddDate(0, 0, -7)
    result, err := svc.Scan(&dynamodb.ScanInput{
        TableName: aws.String(tableName),
    })
    if err != nil {
        return nil, err
    }

    var items []InsightData
    err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
    if err != nil {
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

func getDiscordIDAndTimes(discordID string) ([]InsightData, error) {
    filt := expression.Key("discord_id").Equal(expression.Value(discordID))
    expr, err := expression.NewBuilder().WithKeyCondition(filt).Build()
    if err != nil {
        return nil, err
    }

    result, err := svc.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(tableName),
        KeyConditionExpression:    expr.KeyCondition(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    })
    if err != nil {
        return nil, err
    }

    var items []InsightData
    err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
    if err != nil {
        return nil, err
    }

    return items, nil
}

func extractTimesFromItems(items []InsightData) []time.Time {
    var times []time.Time
    for _, item := range items {
        t, err := time.Parse(time.RFC3339, item.Timestamp)
        if err != nil {
            log.Fatalf("Failed to parse timestamp: %s", err)
        }
        times = append(times, t)
    }
    return times
}

func calculateSessionTimes(times []time.Time) []struct {
    Start time.Time
    End   time.Time
} {
    var sessionTimes []struct {
        Start time.Time
        End   time.Time
    }

    if len(times) == 0 {
        return sessionTimes
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

func calculateLanguageTimes(items []InsightData) map[string]time.Duration {
    languageTimes := make(map[string]time.Duration)
    sort.Slice(items, func(i, j int) bool {
        return items[i].Timestamp < items[j].Timestamp
    })

    var sessionStart time.Time
    for i, item := range items {
        timestamp, err := time.Parse(time.RFC3339, item.Timestamp)
        if err != nil {
            log.Fatalf("Failed to parse timestamp: %s", err)
        }

        if i == 0 || timestamp.Sub(sessionStart) > 5*time.Minute {
            sessionStart = timestamp
        }

        if i == len(items)-1 || items[i+1].Language != item.Language {
            sessionEnd := timestamp
            duration := sessionEnd.Sub(sessionStart)
            languageTimes[item.Language] += duration
        }
    }

    return languageTimes
}

// Prefix to identify roles created by the bot
const rolePrefix = ""
const roleSuffix = "勉強中🔥"

// List of languages to exclude from role assignment
var excludedLanguages = []string{"json", "markdown"} // Replace with actual languages to exclude

func assignRoles(sortedData []DiscordWorkTime) error {
    dg, err := discordgo.New("Bot " + Token)
    if err != nil {
        return fmt.Errorf("error creating Discord session: %w", err)
    }
    defer dg.Close()

    err = dg.Open()
    if err != nil {
        return fmt.Errorf("error opening connection: %w", err)
    }

    // Delete existing roles created by the bot
    err = deleteBotCreatedRoles(dg)
    if err != nil {
        log.Printf("Failed to delete existing roles: %v", err)
    }

    for _, entry := range sortedData {
        for language, duration := range entry.LanguageTimes {
            if isExcludedLanguage(language) {
                continue
            }
            if duration > 60*time.Minute {
                roleID, err := ensureRoleExists(dg, language)
                if err != nil {
                    log.Printf("Failed to ensure role exists: %v", err)
                    continue
                }
                err = dg.GuildMemberRoleAdd(GuildID, entry.DiscordID, roleID)
                if err != nil {
                    log.Printf("Failed to assign role: %v", err)
                }
            }
        }
    }

    return nil
}

// Helper function to check if a language is excluded
func isExcludedLanguage(language string) bool {
    for _, excludedLanguage := range excludedLanguages {
        if language == excludedLanguage {
            return true
        }
    }
    return false
}

// Ensure the role exists, creating it if necessary
func ensureRoleExists(dg *discordgo.Session, language string) (string, error) {
    roles, err := dg.GuildRoles(GuildID)
    if err != nil {
        return "", fmt.Errorf("failed to get roles: %w", err)
    }

    roleName := rolePrefix + language + roleSuffix
    for _, role := range roles {
        if role.Name == roleName {
            return role.ID, nil
        }
    }

    blueColor := 0x0000FF

    roleParams := &discordgo.RoleParams{
        Name:  roleName,
        Color: &blueColor,
    }

    role, err := dg.GuildRoleCreate(GuildID, roleParams)
    if err != nil {
        return "", fmt.Errorf("failed to create role: %w", err)
    }

    return role.ID, nil
}

// Delete roles created by the bot
func deleteBotCreatedRoles(dg *discordgo.Session) error {
    roles, err := dg.GuildRoles(GuildID)
    if err != nil {
        return fmt.Errorf("failed to get roles: %w", err)
    }

    for _, role := range roles {
        if len(role.Name) > len(rolePrefix)+len(roleSuffix) && role.Name[:len(rolePrefix)] == rolePrefix && role.Name[len(role.Name)-len(roleSuffix):] == roleSuffix {
            err = dg.GuildRoleDelete(GuildID, role.ID)
            if err != nil {
                log.Printf("Failed to delete role: %v", err)
            }
        }
    }

    return nil
}
