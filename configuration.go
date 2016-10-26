package main

import "errors"

// Services and app config
type config struct {
	Loggly logglyConfig
	Slack  slackConfig
}

func (c config) valid() error {
	err := c.Loggly.valid()
	if err != nil {
		return err
	}
	err = c.Slack.valid()
	if err != nil {
		return err
	}
	return nil
}

// loggly connection config
type logglyConfig struct {
	account  string
	user     string
	password string
}

func (l logglyConfig) valid() error {
	if len(l.account) == 0 || len(l.user) == 0 || len(l.password) == 0 {
		return errors.New("Loggly configuration incorrect")
	}
	return nil
}

// Slack config
type slackConfig struct {
	token string
}

func (s slackConfig) valid() error {
	if len(s.token) == 0 {
		return errors.New("Missing SLACK_TOKEN")
	}
	return nil
}

// Config struct to load toml search configs
type tomlConfig struct {
	Searches map[string]searchConfig
}

// configuration of an individual search
type searchConfig struct {
	Query            string
	Title            string
	SlackChannel     string
	FrequencyMinutes uint64
	Daily            bool   //If true, FrequencyMinutes needs to be 0 or not defined in the toml
	Time             string // eg, "9:00"
	WindowMinutes    int
	Threshold        int //If more than these numbre of results in the search, it goes to slack
}

func (t tomlConfig) valid() error {

	for _, entry := range t.Searches {
		if entry.Daily {
			if len(entry.Time) == 0 || entry.FrequencyMinutes != 0 {
				return errors.New("A daily entry needs Time and not a FrequencyMinutes")
			}
		} else {
			if len(entry.Time) != 0 || entry.FrequencyMinutes == 0 {
				return errors.New("Missing FrequencyMinutes or Time is defined")
			}
		}
	}
	return nil
}
