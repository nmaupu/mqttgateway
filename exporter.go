package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nmaupu/mqttgateway/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/tidwall/gjson"
)

var (
	mutex            sync.RWMutex
	metricLabelNames = []string{"job", "type", "metric", "topic"}
	mqttClient       mqtt.Client
	exporter         *mqttExporter
)

const (
	mqttMaxReconnectInterval = 30 * time.Second
)

type mqttExporter struct {
	client         mqtt.Client
	versionDesc    *prometheus.Desc
	connectDesc    *prometheus.Desc
	metrics        map[string]*prometheus.GaugeVec   // holds the metrics collected
	counterMetrics map[string]*prometheus.CounterVec // holds the metrics collected
}

func newMQTTExporter() *mqttExporter {
	// create a MQTT client
	options := mqtt.NewClientOptions()
	log.Infof("Connecting to %v", *brokerAddress)
	options.AddBroker(*brokerAddress)
	if *username != "" {
		options.SetUsername(*username)
	}
	if *password != "" {
		options.SetPassword(*password)
	}
	if *clientID != "" {
		options.SetClientID(*clientID)
	}
	options.MaxReconnectInterval = mqttMaxReconnectInterval
	options.ConnectRetry = true
	options.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		log.Infof("Trying to reconnect to MQTT server %+v", options.Servers)
	}
	options.OnConnectionLost = func(client mqtt.Client, reason error) {
		log.Infof("MQTT connection has been lost, reason=%+v", reason)
	}
	options.OnConnect = func(client mqtt.Client) {
		log.Debugf("OnConnectHandler func called")
		log.Debugf("Subscribing to topic %s", *topic)
		mqttClient.Subscribe(*topic, 2, exporter.receiveMessage())
	}

	mqttClient = mqtt.NewClient(options)
	exporter = &mqttExporter{
		client: mqttClient,
		versionDesc: prometheus.NewDesc(
			prometheus.BuildFQName(Progname, "build", "info"),
			"Build info of this instance",
			nil,
			prometheus.Labels{"version": version}),
		connectDesc: prometheus.NewDesc(
			prometheus.BuildFQName(Progname, "mqtt", "connected"),
			"Is the exporter connected to mqtt broker",
			nil,
			nil),
	}

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	exporter.metrics = make(map[string]*prometheus.GaugeVec)
	exporter.counterMetrics = make(map[string]*prometheus.CounterVec)

	return exporter
}

func (c *mqttExporter) Describe(ch chan<- *prometheus.Desc) {
	mutex.RLock()
	defer mutex.RUnlock()
	ch <- c.versionDesc
	ch <- c.connectDesc
	for _, m := range c.counterMetrics {
		m.Describe(ch)
	}
	for _, m := range c.metrics {
		m.Describe(ch)
	}
}

func (c *mqttExporter) Collect(ch chan<- prometheus.Metric) {
	mutex.RLock()
	defer mutex.RUnlock()
	ch <- prometheus.MustNewConstMetric(
		c.versionDesc,
		prometheus.GaugeValue,
		1,
	)
	connected := 0.
	if c.client.IsConnected() {
		connected = 1.
	}
	ch <- prometheus.MustNewConstMetric(
		c.connectDesc,
		prometheus.GaugeValue,
		connected,
	)
	for _, m := range c.counterMetrics {
		m.Collect(ch)
	}
	for _, m := range c.metrics {
		m.Collect(ch)
	}
}

/*
Topic is Tasmota MQTT telemetry format (with setoption19 enabled - home assistant discovery) so needs the following FullTopic format: %topic%/%prefix%/<command>
As explained here: https://github.com/arendst/Tasmota/wiki/Home-Assistant
device_name/tele/SENSOR = {"Time":"2019-11-19T00:20:06","AM2301":{"Temperature":20.5,"Humidity":44.4},"TempUnit":"C"}
job/type/metric = json_data
*/
func (e *mqttExporter) receiveMessage() func(mqtt.Client, mqtt.Message) {
	return func(c mqtt.Client, m mqtt.Message) {
		mutex.Lock()
		defer mutex.Unlock()

		topic := m.Topic()

		// Match if topic is in config
		configMqttTopic := Config.GetTopic(topic)
		if configMqttTopic == nil {
			log.Debugf("Topic %s does not match any conf, skipping. value=%+v\n", topic, string(m.Payload()))
			return
		}

		log.Debugf("%s matches config, processing it (value = %s)\n", topic, string(m.Payload()))

		// Get patterns to get
		gjsonPatterns := configMqttTopic.GetAllPatterns()
		for _, v := range gjsonPatterns {
			e.processPattern(m, v)
		}
	}
}

func (e *mqttExporter) processPattern(message mqtt.Message, p conf.ConfigGjsonPattern) {
	topic := message.Topic()
	parts := strings.Split(topic, "/")
	if len(parts) != 3 {
		log.Warnf("Invalid topic ! %s: number of levels is not 3, ignoring", topic)
		return
	}

	jobName := parts[0]
	topicType := parts[1]
	metricType := parts[2]

	mapIndex := fmt.Sprintf("%s-%s", topic, p.Name)
	metricName := p.Name
	labelValues := prometheus.Labels{}
	labelValues["job"] = jobName
	labelValues["type"] = topicType
	labelValues["metric"] = metricType
	labelValues["topic"] = topic

	mqttValue, err := strconv.ParseFloat(
		gjson.GetBytes(message.Payload(), p.Pattern).String(), 32)
	if err == nil {
		log.Debugf("Creating new metric: %s %+v\n", metricName, metricLabelNames)
		e.metrics[mapIndex] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: "Metric pushed via MQTT",
			},
			metricLabelNames,
		)

		countMetricName := "mqtt_push_total"
		e.counterMetrics[countMetricName] = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: countMetricName,
				Help: fmt.Sprintf("Number of times metric has been pushed via MQTT"),
			},
			metricLabelNames,
		)

		lastPushTimeMetricName := "mqtt_last_pushed_timestamp"
		e.metrics[lastPushTimeMetricName] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: lastPushTimeMetricName,
				Help: fmt.Sprintf("Last time metric was pushed via MQTT"),
			},
			metricLabelNames,
		)

		log.Debugf("%s for %s = %.1f", p.Name, jobName, mqttValue)
		e.metrics[mapIndex].With(labelValues).Set(mqttValue)
		e.metrics[lastPushTimeMetricName].With(labelValues).SetToCurrentTime()
		e.counterMetrics[countMetricName].With(labelValues).Inc()
	}
}
