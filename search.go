package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/inconshreveable/log15"
	"github.com/jasonlvhit/gocron"
	"github.com/nlopes/slack"
	"github.com/olekukonko/tablewriter"
	search "github.com/segmentio/go-loggly-search"
)

func env(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Error("missing-env-var", "var", name)
	}
	return val
}

func main() {

	configFile := os.Getenv("CONFIG_FILE")
	if len(configFile) == 0 {
		configFile = "searches.toml"
	}
	log.Info("logglum-init", "config", configFile)

	logglyConf := logglyConfig{
		account:  env("LOGGLY_ACCOUNT"),
		user:     env("LOGGLY_USER"),
		password: env("LOGGLY_PASSWORD"),
	}

	slackConf := slackConfig{token: env("SLACK_TOKEN")}

	configuration := config{Loggly: logglyConf, Slack: slackConf}

	var searches tomlConfig

	_, err := toml.DecodeFile(configFile, &searches)
	if err != nil {
		log.Error("config-toml-decode", "error", err)
		return
	}
	err = searches.valid()
	if err != nil {
		log.Error("config-toml-validate", "error", err)
		return
	}

	err = configuration.valid()
	if err != nil {
		log.Error("invalid-config-env-vars", "error", err)
		return
	}

	for _, search := range searches.Searches {
		if search.Daily {
			gocron.Every(1).Day().At(search.Time).Do(executeQuery, search, configuration)
		} else {
			gocron.Every(search.FrequencyMinutes).Minutes().Do(executeQuery, search, configuration)
		}
	}

	<-gocron.Start()

}

func executeQuery(search searchConfig, appConfig config) {

	slackObj := slack.New(appConfig.Slack.token)

	now := time.Now().UTC()

	log.Debug("query-exec-start", "title", search.Title, "time", now)

	windowStart := now.Add(time.Duration(search.WindowMinutes*-1) * time.Minute)
	groupedMsgs, total := summarizeLogglyQuery(search.Query, windowStart.String(), appConfig.Loggly)
	if total >= search.Threshold {
		if total == 0 { // Just in case threshold is 0 we want a text to notify that it is empty
			groupedMsgs = "No results"
		}
		params := getSlackMessage(groupedMsgs, search.Query, windowStart.String(), now.String(), search.Title, search.SlackChannel)
		channelID, timestamp, err := slackObj.PostMessage(search.SlackChannel, "", params)
		if err != nil {
			log.Error("query-exec-slack-error", "error", err)
			return
		}
		log.Debug("query-exec-notified", "channel", channelID, "timestamp", timestamp)
	} else {
		log.Debug("query-exec-below-thresold", "thresold", search.Threshold, "total", total)
	}
}

func getSlackMessage(message string, query string, since string, to string, title string, channel string) slack.PostMessageParameters {

	params := slack.PostMessageParameters{Username: "logglum", AsUser: true}

	fields := make([]slack.AttachmentField, 1)
	fields[0].Value = "```" + message + "```"
	fields[0].Short = false
	attachment := slack.Attachment{
		Color:      "#ff0000",
		Fields:     fields,
		MarkdownIn: []string{"fields"},
		Title:      title,
		TitleLink:  "https://comptel.loggly.com/search#terms=" + url.QueryEscape(query) + "&from=" + url.QueryEscape(since) + "&to=" + url.QueryEscape(to),
	}
	params.Attachments = []slack.Attachment{attachment}
	return params
}

// struct used to decode the json message from loggly (the body of the log entry)
type logglyEntry struct {
	Msg string
	Lvl string
}

// make a query to loggly and get a summary back, in nicely formated table format, and the number of events
func summarizeLogglyQuery(query string, period string, loggly logglyConfig) (string, int) {

	c := search.New(loggly.account, loggly.user, loggly.password)

	var summaryMap map[string]int
	summaryMap = make(map[string]int)

	res, err := c.Query(query).Size(5000).From(period).Fetch()
	if err != nil {
		log.Error("loggly-query-error", "error", err)
		return "", 0
	}

	for _, event := range res.Events {
		entryRaw := event.(map[string]interface{})["logmsg"]
		entry := logglyEntry{}
		json.Unmarshal([]byte(entryRaw.(string)), &entry)
		current := summaryMap[entry.Msg]
		summaryMap[entry.Msg] = current + 1

	}
	//Generate a nice table view
	outputBuffer := new(bytes.Buffer)
	table := tablewriter.NewWriter(outputBuffer)
	table.SetColumnSeparator(" ")
	table.SetBorder(false)
	total := 0

	for k, count := range summaryMap {
		table.Append([]string{k, strconv.Itoa(count)})
		total = total + count
	}

	table.Render()
	return outputBuffer.String(), total

}
