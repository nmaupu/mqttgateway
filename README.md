# MQTTGateway for Prometheus

A project that subscribes to MQTT queues and published prometheus metrics.
Compatible with Tasmota firmware.

```
usage: mqttgateway [<flags>]

Flags:
  --help                        Show context-sensitive help (also try --help-long and --help-man).
  --web.listen-address=":9337"  Address on which to expose metrics and web interface.
  --web.telemetry-path="/metrics"
                                Path under which to expose metrics.
  --mqtt.broker-address="tcp://localhost:1883"
                                Address of the MQTT broker.
                                The default is taken from $MQTT_BROKER_ADDRESS if it is set.
  --mqtt.topic="prometheus/#"   MQTT topic to subscribe to.
                                The default is taken from $MQTT_TOPIC if it is set.
  --mqtt.prefix="prometheus"    MQTT topic prefix to remove when creating metrics.
                                The default is taken from $MQTT_PREFIX if it is set.
  --mqtt.username=""            MQTT username.
                                The default is taken from $MQTT_USERNAME if it is set.
  --mqtt.password=""            MQTT password.
                                The default is taken from $MQTT_PASSWORD if it is set.
  --mqtt.clientid=""            MQTT client ID.
                                The default is taken from $MQTT_CLIENT_ID if it is set.
  --log.level="info"            Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
  --log.format="logger:stderr"  Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
```

## Installation

### Go get

Requires go > 1.9

```
go get -u github.com/nmaupu/mqttgateway
```

### Docker

```
docker run -v `pwd`/mqttgateway-config.yaml:/etc/mqttgateway/mqttgateway-config.yaml nmaupu/mqttgateway --mqtt.broker-address=tcp://192.168.12.1:1883 --mqtt.prefix="" --mqtt.username=mqtt --mqtt.password=mqtt --mqtt.clientid=mqttgateway-dockertest --log.level=debug --mqtt.topic="#"
```

Make sure, `mqtt.clientid` is uniq !

## How does it work?

mqttgateway will connect to the MQTT broker at `--mqtt.broker-address` and
listen to the topics specified by `--mqtt.topic`.

By default, it will listen to `#`.

The format for the topics is as follow:

`device_name/{tele,stats,cmnd}/metric_type`
e.g. `device_name/tele/SENSOR`

Content has to be in JSON for mqttgateway to work !

Json parsing is configured using a configuration file named `mqttgateway-config.yaml` in the following possible locations:
- `/etc/mqttgateway`
- `$HOME/.mqttgateway`
- current working directory

When starting, mqttgateway will tell you what configuration it loaded.

Configuration is pretty straightforward and is as follow:
```
topics:
  - topic: A topic to subscribe to
    patterns:
  - name: name of the metric in prometheus
    pattern: Gjson pattern to look for in the payload
```

Example:
When a message like this is retrieved:
```
living/tele/SENSOR = {"Time":"2019-11-25T01:23:17","AM2301":{"Temperature":20.4,"Humidity":43.9},"TempUnit":"C"}
```
One can use the following configuration to parse it and add metrics to prometheus:
```
topics:
  - topic: living/tele/SENSOR
    patterns:
    - name: temperature
      pattern: "*.Temperature"
    - name: humidity
      pattern: "*.Humidity"
```

All of this is case sensitive !

Two other metrics are published, for each metric:

- `mqtt_NAME_last_pushed_timestamp`, the last time NAME metric has been pushed
(unix time, in seconds)
- `mqtt_NAME_push_total`, the number of times a metric has been pushed

## Security

This project does not support authentication yet.

## A note about the prometheus config

If you use `job` and `instance` labels, please refer to the [pushgateway
exporter
documentation](https://github.com/prometheus/pushgateway#about-the-job-and-instance-labels).

TL;DR: you should set `honor_labels: true` in the scrape config.
