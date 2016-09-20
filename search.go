package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"

	"net/url"

	"time"

	log "github.com/inconshreveable/log15"
	"github.com/jasonlvhit/gocron"
	"github.com/nlopes/slack"
	"github.com/olekukonko/tablewriter"
	search "github.com/segmentio/go-loggly-search"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func env(name string) string {
	val := os.Getenv(name)

	if val == "" {
		log.Error("missing-env-var", "var", name)
	}

	return val
}

type logglyInfo struct {
	account  string
	user     string
	password string
}

func (l logglyInfo) valid() bool {

	return len(l.account) > 0 && len(l.user) > 0 && len(l.password) > 0
}

func main() {

	loggly := logglyInfo{}

	loggly.account = env("ACCOUNT")
	loggly.user = env("USER")
	loggly.password = env("PASS")

	slackToken := env("SLACK_TOKEN")
	slackChannelID := env("SLACK_CHANNEL_ID")

	if len(slackToken) == 0 || len(slackChannelID) == 0 {
		log.Error("invalid-slack-env-vars")
		return
	}

	if loggly.valid() {

		gocron.Every(1).Day().At("09:00").Do(func() { dailySummary(loggly, slackToken, slackChannelID) })
		gocron.Every(30).Minutes().Do(func() { hourlyCheck(loggly, slackToken, slackChannelID) }) // the sample rate needs to be double

		<-gocron.Start()

	} else {
		log.Error("invalid-loggly-env-vars")
		return
	}
}

func dailySummary(loggly logglyInfo, slackToken string, channel string) {

	slackObj := slack.New(slackToken)

	now := time.Now().UTC()

	log.Debug("daily-summary-start", "time", now)

	query := `tag:"ctrl" and not tag:"st-eve-syslog" and not tag:"st-abel-syslog" and (derived.lvl:"warn" or derived.lvl:"eror") and not derived.msg:"smpl-uploaded-error"`

	//24h ago
	yesterday := now.Add(-24 * time.Hour)
	groupedMsgs, _ := summarizeLogglyQuery(query, yesterday.String(), loggly)

	params := getSlackMessage(groupedMsgs, query, yesterday.String(), "Daily Summary", channel)
	channelID, timestamp, err := slackObj.PostMessage(channel, "", params)
	if err != nil {
		log.Error("daily-summary-slack-error", "error", err)
		return
	}
	log.Debug("daily-summary-slack", "channel", channelID, "timestamp", timestamp)
}

func hourlyCheck(loggly logglyInfo, slackToken string, channel string) {
	slackObj := slack.New(slackToken)

	now := time.Now().UTC()

	log.Debug("hourly-summary-start", "time", now)

	query := `tag:"ctrl" and not tag:"st-eve-syslog" and not tag:"st-abel-syslog" and (derived.lvl:"warn" or derived.lvl:"eror") and not derived.msg:"smpl-uploaded-error"`

	//1h ago
	onehourago := now.Add(-1 * time.Hour)
	groupedMsgs, total := summarizeLogglyQuery(query, onehourago.String(), loggly)

	if total > 50 {
		params := getSlackMessage(groupedMsgs, query, onehourago.String(), "Hourly warning", channel)
		channelID, timestamp, err := slackObj.PostMessage(channel, "", params)
		if err != nil {
			log.Error("hourly-summary-slack-error", "error", err)
			return
		}
		log.Debug("hourly-summary-slack", "channel", channelID, "timestamp", timestamp)
	} else {
		log.Debug("hourly-summary-nothing-to-alert")
	}
}

func getSlackMessage(groups []envSummary, query string, since string, title string, channel string) slack.PostMessageParameters {

	params := slack.PostMessageParameters{Username: "logglum", AsUser: true}

	fields := make([]slack.AttachmentField, len(groups))
	for i, v := range groups {
		fields[i].Title = v.environment
		fields[i].Value = "```" + v.summaries + "```"
		fields[i].Short = false
	}
	attachment := slack.Attachment{
		Color:      "#ff0000",
		Fields:     fields,
		MarkdownIn: []string{"fields"},
		Title:      title,
		TitleLink:  "https://comptel.loggly.com/search#terms=" + url.QueryEscape(query) + "&from=" + url.QueryEscape(since),
	}
	params.Attachments = []slack.Attachment{attachment}
	return params
}

type envSummary struct {
	environment string
	summaries   string
}

// make a query to loggly and get a summary back, in nicely formated table format, and the number of events
func summarizeLogglyQuery(query string, period string, loggly logglyInfo) ([]envSummary, int) {

	c := search.New(loggly.account, loggly.user, loggly.password)

	outputBuffer := new(bytes.Buffer)

	var summaryMap map[string]int
	summaryMap = make(map[string]int)

	var envMap map[string]bool
	envMap = make(map[string]bool)

	res, err := c.Query(query).Size(5000).From(period).Fetch()
	check(err)

	counter := len(res.Events)
	for _, event := range res.Events {
		tag, message := summary(event)
		key := tag + "." + message
		current := summaryMap[key]
		summaryMap[key] = current + 1

		//add it to the map of envs
		envMap[tag] = true
	}

	//Iterate per env

	groups := []envSummary{}

	for env := range envMap {

		grouped := envSummary{environment: env}

		//Very naive implementation, but it works :)
		// fmt.Fprintln(outputBuffer, env)
		table := tablewriter.NewWriter(outputBuffer)
		table.SetColumnSeparator(" ")
		table.SetBorder(false)
		for k, count := range summaryMap {
			if strings.Contains(k, env) {
				// it is part of this env
				message := strings.Replace(k, env+".", "", 1)
				table.Append([]string{message, strconv.Itoa(count)})
				// fmt.Println(message, "\t", count)
			}
		}
		table.Render()
		grouped.summaries = outputBuffer.String()
		groups = append(groups, grouped)
		outputBuffer.Reset()

	}

	return groups, counter

}

// from a loggly event, get the summary and the tag
func summary(event interface{}) (string, string) {
	msg := event.(map[string]interface{})
	msgEvent := msg["event"].(map[string]interface{})
	eventDerived := msgEvent["derived"].(map[string]interface{})

	tag := getTag(msg["tags"])

	return tag, eventDerived["msg"].(string)
}

// Get the tag of the event, it should have the `-syslog` as it will be the env
func getTag(tags interface{}) string {

	tagAr := tags.([]interface{})
	for _, tagInt := range tagAr {
		tag := tagInt.(string)
		if strings.Contains(tag, "-syslog") {
			return strings.Replace(tag, "-syslog", "", 1)
		}
	}

	return ""
}
