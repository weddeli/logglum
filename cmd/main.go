package main

import (
	"github.com/BurntSushi/toml"
	log "github.com/inconshreveable/log15"
	"github.com/jasonlvhit/gocron"
	"logglum"
	"math/rand"
	"os"
	"strconv"
	"time"
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

	jsonLogsRaw := os.Getenv("JSON_LOGS")
	if len(jsonLogsRaw) > 0 {
		stdoutHandler := log.BufferedHandler(200000, log.StreamHandler(os.Stdout, log.JsonFormat()))
		log.Root().SetHandler(stdoutHandler)
	}

	logglyConf := logglum.LogglyConfig{
		Account:  env("LOGGLY_ACCOUNT"),
		Token:     env("LOGGLY_TOKEN"),
	}

	slackConf := logglum.SlackConfig{Token: env("SLACK_TOKEN")}

	configuration := logglum.Config{Loggly: logglyConf, Slack: slackConf}

	var searches logglum.TomlConfig

	_, err := toml.DecodeFile(configFile, &searches)
	if err != nil {
		log.Error("config-toml-decode", "errorString", err)
		return
	}
	err = searches.Valid()
	if err != nil {
		log.Error("config-toml-validate", "errorString", err)
		return
	}

	err = configuration.Valid()
	if err != nil {
		log.Error("invalid-config-env-vars", "errorString", err)
		return
	}

	// start the random with something that at least changes with time.
	// we don't care if it is fully random, so more than enough for our needs
	rand.Seed(int64(time.Now().Nanosecond()))

	for _, search := range searches.Searches {

		if search.Daily {
			randomness := strconv.Itoa(int(rand.Int31n(8)))       // 8 mins in string
			time := search.Time[:len(search.Time)-1] + randomness // we have the time in format 09:00 and we replace the last char with some random value
			gocron.Every(1).Day().At(time).Do(logglum.ExecuteQuery, search, configuration)
		} else {
			gocron.Every(search.FrequencyMinutes).Minutes().Do(logglum.ExecuteQuery, search, configuration)
		}
	}

	<-gocron.Start()

}
