package main

import (
    "fmt"
    "log"
    "os"
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
    discordIDMap, err := getUniqueDiscordIDs()
    if err != nil {
        log.Fatalf("Failed to get unique Discord IDs: %s", err)
        return nil
    }

    var data []DiscordWorkTime
    for discordID, _ := range discordIDMap {
        times, languages, err := getDiscordIDAndTimes(discordID)
        if err != nil {
            log.Printf("Failed to get data for Discord ID: %s", discordID)
            continue
        }

        if len(times) > 0 {
            sessionTimes, languageDurations := calculateSessionTimes(times, languages)
            totalWorkTime := getTotalWorkTime(sessionTimes)
            data = append(data, DiscordWorkTime{
                DiscordID:    discordID,
                TotalTime:    totalWorkTime,
                LanguageTimes: languageDurations,
            })
        }
    }

    sort.Slice(data, func(i, j int) bool {
        return data[i].TotalTime > data[j].TotalTime
    })

    return data
}

func getUniqueDiscordIDs() (map[string]string, error) {
    now := time.Now().UTC()
    sevenDaysAgo := now.AddDate(0, 0, -7)
    startDate := time.Date(
        sevenDaysAgo.Year(),
        sevenDaysAgo.Month(),
        sevenDaysAgo.Day(),
        0, 0, 0, 0,
        sevenDaysAgo.Location(),
    )

    filt := expression.Name("timestamp").GreaterThanEqual(expression.Value(startDate.Format(time.RFC3339)))
    expr, err := expression.NewBuilder().WithFilter(filt).Build()
    if err != nil {
        return nil, err
    }

    result, err := svc.Scan(&dynamodb.ScanInput{
        TableName:                 aws.String(tableName),
        FilterExpression:          expr.Filter(),
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

    uniqueDiscordIDs := make(map[string]string)
    for _, item := range items {
        uniqueDiscordIDs[item.DiscordID] = item.DiscordID
    }

    return uniqueDiscordIDs, nil
}

func getDiscordIDAndTimes(discordID string) ([]time.Time, []string, error) {
    now := time.Now().UTC()
    sevenDaysAgo := now.AddDate(0, 0, -7)
    startDate := time.Date(
        sevenDaysAgo.Year(),
        sevenDaysAgo.Month(),
        sevenDaysAgo.Day(),
        0, 0, 0, 0,
        sevenDaysAgo.Location(),
    )

    keyCond := expression.Key("discord_id").Equal(expression.Value(discordID)).
        And(expression.Key("timestamp").GreaterThanEqual(expression.Value(startDate.Format(time.RFC3339))))

    builder := expression.NewBuilder().WithKeyCondition(keyCond)
    expr, err := builder.Build()
    if err != nil {
        return nil, nil, err
    }

    result, err := svc.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(tableName),
        KeyConditionExpression:    expr.KeyCondition(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    })
    if err != nil {
        return nil, nil, err
    }

    var items []InsightData
    err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &items)
    if err != nil {
        return nil, nil, err
    }

    var times []time.Time
    var languages []string
    for _, item := range items {
        t, err := time.Parse(time.RFC3339, item.Timestamp)
        if err != nil {
            log.Printf("Failed to parse timestamp: %v", err)
            continue
        }
        times = append(times, t)
        languages = append(languages, item.Language)
    }

    sort.Slice(times, func(i, j int) bool {
        return times[i].Before(times[j])
    })

    return times, languages, nil
}

func calculateSessionTimes(times []time.Time, languages []string) ([]struct {
    Start time.Time
    End   time.Time
}, map[string]time.Duration) {
    var sessionTimes []struct {
        Start time.Time
        End   time.Time
    }
    languageDurations := make(map[string]time.Duration)

    if len(times) == 0 {
        return sessionTimes, languageDurations
    }

    sessionStart := times[0]
    sessionEnd := times[0]
    currentLanguage := languages[0]

    for i := 1; i < len(times); i++ {
        if times[i].Sub(sessionEnd) > 5*time.Minute {
            sessionTimes = append(sessionTimes, struct {
                Start time.Time
                End   time.Time
            }{Start: sessionStart, End: sessionEnd})
            languageDurations[currentLanguage] += sessionEnd.Sub(sessionStart)
            sessionStart = times[i]
            currentLanguage = languages[i]
        }
        sessionEnd = times[i]
    }

    sessionTimes = append(sessionTimes, struct {
        Start time.Time
        End   time.Time
    }{Start: sessionStart, End: sessionEnd})
    languageDurations[currentLanguage] += sessionEnd.Sub(sessionStart)

    return sessionTimes, languageDurations
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

// Prefix to identify roles created by the bot
const rolePrefix = ""
const roleSuffix = "勉強中🔥"

// List of languages to exclude from role assignment
var excludedLanguages = []string{"json", "markdown"} // Replace with actual languages to exclude

func assignRoles(sortedData []DiscordWorkTime) error {
    discordToken := os.Getenv("DISCORD_TOKEN")
    guildID := os.Getenv("DISCORD_GUILD_ID")

    if discordToken == "" || guildID == "" {
        return fmt.Errorf("DISCORD_TOKEN or DISCORD_GUILD_ID environment variable is not set")
    }

    dg, err := discordgo.New("Bot " + discordToken)
    if err != nil {
        return fmt.Errorf("error creating Discord session: %w", err)
    }
    defer dg.Close()

    err = dg.Open()
    if err != nil {
        return fmt.Errorf("error opening connection: %w", err)
    }

    // Delete existing roles created by the bot
    err = deleteBotCreatedRoles(dg, guildID)
    if err != nil {
        log.Printf("Failed to delete existing roles: %v", err)
    }

    for _, entry := range sortedData {
        for language, duration := range entry.LanguageTimes {
            if isExcludedLanguage(language) {
                continue
            }
            if duration > 60*time.Minute {
                roleID, err := ensureRoleExists(dg, guildID, language)
                if err != nil {
                    log.Printf("Failed to ensure role exists: %v", err)
                    continue
                }
                err = dg.GuildMemberRoleAdd(guildID, entry.DiscordID, roleID)
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
func ensureRoleExists(dg *discordgo.Session, guildID, language string) (string, error) {
    roles, err := dg.GuildRoles(guildID)
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

    role, err := dg.GuildRoleCreate(guildID, roleParams)
    if err != nil {
        return "", fmt.Errorf("failed to create role: %w", err)
    }

    return role.ID, nil
}

// Delete roles created by the bot
func deleteBotCreatedRoles(dg *discordgo.Session, guildID string) error {
    roles, err := dg.GuildRoles(guildID)
    if err != nil {
        return fmt.Errorf("failed to get roles: %w", err)
    }

    for _, role := range roles {
        if len(role.Name) > len(rolePrefix)+len(roleSuffix) && role.Name[:len(rolePrefix)] == rolePrefix && role.Name[len(role.Name)-len(roleSuffix):] == roleSuffix {
            err = dg.GuildRoleDelete(guildID, role.ID)
            if err != nil {
                log.Printf("Failed to delete role: %v", err)
            }
        }
    }

    return nil
}