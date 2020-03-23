package logglum

import (
	"bytes"
	"encoding/json"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/nlopes/slack"
	"github.com/olekukonko/tablewriter"
)


// The max of the results per loggly search. TODO Move this to the Config file or per search value
const maxLogglyResults = 5000

func ExecuteQuery(search searchConfig, appConfig Config) {


	now := time.Now().UTC()

	log.Debug("query-exec-start", "title", search.Title, "time", now)

	windowStart := now.Add(time.Duration(search.WindowMinutes*-1) * time.Minute)
	groupedMsgs, total := summarizeLogglyQueryRetrier(search.Title, search.Query, windowStart.String(), appConfig.Loggly)
	if total >= search.Threshold {
		if total == 0 { // Just in case threshold is 0 we want a text to notify that it is empty
			groupedMsgs = "No results"
		}
		params := getSlackMessage(groupedMsgs, search.Query, windowStart.String(), now.String(), search.Title+" "+strconv.Itoa(total), search.SlackChannel, appConfig.Loggly.Account)
		for _, item := range params {
			err := slack.PostWebhook(appConfig.Slack.WebhookURL,  item)
			if err != nil {
				log.Error("query-exec-slack-error", "errorString", err)
				return
			}
			log.Debug("query-exec-notified")
		}
	} else {
		log.Debug("query-exec-below-threshold", "threshold", search.Threshold, "total", total)
	}
}

func getSlackMessage(message string, query string, since string, to string, title string, channel string, looglyAccount string) []*slack.WebhookMessage {

	round := func(a float64) int {
		if a < 0 {
			return int(math.Ceil(a - 0.5))
		}
		return int(math.Floor(a + 0.5))
	}
	const maxLinesSlackMessage = 26.0 // you cannot post more than 26 lines in an attachment in slack

	lines := strings.Split(message, "\n")
	numberLines := len(lines)

	messagesNeeded := round(float64(numberLines) / maxLinesSlackMessage)
	if messagesNeeded == 0 {
		messagesNeeded = 1 //we need at least one :)
	}

	result := make([]*slack.WebhookMessage, messagesNeeded)

	for i := range result {
		params := &slack.WebhookMessage{Username: "logglum"}

		initRange := i * maxLinesSlackMessage
		endRange := (i + 1) * maxLinesSlackMessage
		if endRange > numberLines {
			endRange = numberLines
		}
		messageLines := strings.Join(lines[initRange:endRange], "\n")
		fields := make([]slack.AttachmentField, 1)
		fields[0].Value = "```\n" + messageLines + "```"
		fields[0].Short = false
		attachment := slack.Attachment{
			Color:      "#ff0000",
			Fields:     fields,
			MarkdownIn: []string{"fields"},
			Title:      title,
			TitleLink:  "https://" + looglyAccount + ".loggly.com/search#terms=" + url.PathEscape(query) + "&from=" + url.PathEscape(since) + "&until=" + url.PathEscape(to),
		}
		params.Attachments = []slack.Attachment{attachment}
		result[i] = params
	}

	return result
}

// struct used to decode the json message from loggly (the body of the log entry)
type logglyEntry struct {
	Msg string
	Lvl string
}


// summarizeLogglyQueryRetrier make a query to loggly and get a summary back, in nicely formated table format, and the number of events,
// if the query fails it retries once, as loggly API is crappy and fails many times
func summarizeLogglyQueryRetrier(title, query string, period string, loggly LogglyConfig) (string, int) {

	summary, size, err := summarizeLogglyQuery(title, query, period, loggly)
	if err != nil {
		summary, size, err = summarizeLogglyQuery(title, query, period, loggly)
		if err != nil {
			log.Error("loggly-query-error", "error", err, "name", title)
		}
	}
	return summary, size
}

// summarizeLogglyQuery make a query to loggly and get a summary back, in nicely formated table format, and the number of events
func summarizeLogglyQuery(title, query string, period string, loggly LogglyConfig) (string, int, error) {

	c := NewClient(loggly.Account, loggly.Token)

	summaryMap := make(map[string]int)

	var keys []string //used to sort the map

	res, err := c.Query(query).Size(maxLogglyResults).From(period).Fetch()
	if err != nil {
		return "", 0, err
	}

	for _, event := range res.Events {
		entryRaw := event.(map[string]interface{})["logmsg"]
		entry := logglyEntry{}
		json.Unmarshal([]byte(entryRaw.(string)), &entry)
		current, found := summaryMap[entry.Msg]
		if !found {
			keys = append(keys, entry.Msg)
		}
		summaryMap[entry.Msg] = current + 1
		if len(entry.Msg) == 0 && current == 1 {
			//it is an entry with empty msg, print it in logs to fix the loggin issue
			log.Debug("empty-message-log", "message", entry)
		}

	}
	sort.Strings(keys)

	//Generate a nice table view
	outputBuffer := new(bytes.Buffer)
	table := tablewriter.NewWriter(outputBuffer)
	table.SetColumnSeparator(" ")
	table.SetBorder(false)
	total := 0

	for _, key := range keys {
		table.Append([]string{key, strconv.Itoa(summaryMap[key])})
		total = total + summaryMap[key]
	}

	table.Render()
	return outputBuffer.String(), total, nil

}
