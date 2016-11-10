# logglum

Generate loggly search summaries and forward them to a slack channel with a nice format. Very useful to get daily summaries of what has happened.

![Example Slack output](https://www.fwd.cloud/commit/img/logglum-example.png)

## Why not alerts?

Alerts from loggly only have at most 10 entries in the callback. We wanted to have full summaries of what has happened in our production/staging environments with a fast glance. 


## How it works?

logglum runs as a docker container, and it reads a `toml` file with the configuration of the searches that need to be performed. The file can be somehting like:

```toml
[searches]

  [searches.threshold]
  Query = "json.lvl:\"warn\" or json.lvl:\"eror\""  # The search in loggly
  Title = "Hourly Threshold"        # Title of the notification
  SlackChannel = "C04545454"        # Channel to post notifications to
  FrequencyMinutes = 10             #Should be double of the window you want, remember Shannon theorem
  WindowMinutes = 60
  Threshold = 50

  [searches.summary]
  Query = "json.lvl:\"warn\" or json.lvl:\"eror\""
  Title = "Daily Summary"
  SlackChannel = "C04545454"
  Daily = true
  Time = "09:00"       # Format HH:MM
  WindowMinutes = 1440 # 24h in minutes
  Threshold = 0        # All warnings and errors daily to slack
  
```

which will trigger two searches, one every 10 minutes, and will trigger the slack notification if only there are more than 50 events. The second one it is a daily summary of all warnings and errors.

To be able to run it, you need to pass to logglum the needed credentials, you can use `docker` command line or use a `docker-compose.yml` like the following:

```yaml
version: '2'
services:
    logglum:
        image: comptel/logglum:master
        restart: unless-stopped
        environment:
            LOGGLY_ACCOUNT: …     # your loggly account
            LOGGLY_USER: logglum  # loggly username
            LOGGLY_PASSWORD: …    # loggly password
            SLACK_TOKEN: …        # slack webhook token https://koffee.slack.com/apps/manage/custom-integrations > Incoming Webhooks
            CONFIG_FILE: /etc/logglum/searches.toml
        volumes:
            - /etc/logglum:/etc/logglum   # you can change the source of the config files
```
Then you do a `docker-compose up -d` and it should be running. In case you want to update the config file the docker volume is preserved between runs, so you need to delete the volume to get the new config.

If you want to use `docker` directly it would be something like:

```
docker run -d -e LOGGLY_ACCOUNT="YourLogglyAccount" -e LOGGLY_USER="LogglyAccountUser" -e LOGGLY_PASSWORD="LogglyAccountPassword" -e SLACK_TOKEN="YourSlackToken" -e CONFIG_FILE="/etc/logglum/searches.toml" -v /etc/logglum:/etc/logglum --restart unless-stopped comptel/logglum:master
```

## Other important things

logglum takes a couple of asumptions that you will need in order to work. First is that the format of your logs needs to be in JSON. If you don't already do so, you should, it makes loggly searches more powerful.
Also logglum aggregates the events by the `json.msg` parameter.  All our logs are currently in the format:

```json
{"lvl":"dbug","msg":"redis-timer-locked","t":"2016-10-28T09:45:29.606276309Z"}
```

So we perform searches on `json.lvl` and then we aggragate on `json.msg` to get the summaries.

If you want to change the field used for the aggregation, you can do so by adding the json fields tags to the struct:

```go
type logglyEntry struct {
	Msg string  `json:"yourfield"`
	Lvl string  `json:"yourfields"`
}
```
