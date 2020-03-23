package logglum

import "errors"

// Services and app Config
type Config struct {
	Loggly LogglyConfig
	Slack  SlackConfig
}

func (c Config) Valid() error {
	err := c.Loggly.Valid()
	if err != nil {
		return err
	}
	err = c.Slack.Valid()
	if err != nil {
		return err
	}
	return nil
}

// loggly connection Config
type LogglyConfig struct {
	Account  string
	Token     string
}

func (l LogglyConfig) Valid() error {
	if len(l.Account) == 0 || len(l.Token) == 0  {
		return errors.New("Loggly configuration incorrect")
	}
	return nil
}

// Slack Config
type SlackConfig struct {
	WebhookURL string
}

func (s SlackConfig) Valid() error {
	if len(s.WebhookURL) == 0 {
		return errors.New("Missing SLACK_WEBHOOK")
	}
	return nil
}

// Config struct to load toml search configs
type TomlConfig struct {
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

func (t TomlConfig) Valid() error {

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
