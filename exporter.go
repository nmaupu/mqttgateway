package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/tidwall/gjson"
)

var (
	mutex            sync.RWMutex
	metricLabelNames = []string{"job", "type", "metric", "topic"}
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
	m := mqtt.NewClient(options)
	if token := m.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	// create an exporter
	c := &mqttExporter{
		client: m,
		versionDesc: prometheus.NewDesc(
			prometheus.BuildFQName(progname, "build", "info"),
			"Build info of this instance",
			nil,
			prometheus.Labels{"version": version}),
		connectDesc: prometheus.NewDesc(
			prometheus.BuildFQName(progname, "mqtt", "connected"),
			"Is the exporter connected to mqtt broker",
			nil,
			nil),
	}

	c.metrics = make(map[string]*prometheus.GaugeVec)
	c.counterMetrics = make(map[string]*prometheus.CounterVec)

	m.Subscribe(*topic, 2, c.receiveMessage())

	return c
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
		log.Debugf("Receiving %s = %s\n", topic, string(m.Payload()))
		parts := strings.Split(topic, "/")
		if len(parts) != 3 {
			log.Warnf("Invalid topic ! %s: number of levels is not 3, ignoring", topic)
			return
		}

		jobName := parts[0]
		topicType := parts[1]
		metricType := parts[2]

		metricName := "temperature"
		labelValues := prometheus.Labels{}
		labelValues["job"] = jobName
		labelValues["type"] = topicType
		labelValues["metric"] = metricType
		labelValues["topic"] = topic

		if m, err := strconv.ParseFloat(gjson.GetBytes(m.Payload(), "*.Temperature").String(), 32); err == nil {
			log.Debugf("Creating new metric: %s %v", metricName, metricLabelNames)
			e.metrics[metricName] = prometheus.NewGaugeVec(
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

			log.Debugf("Temperature for %s = %.1f", jobName, m)
			e.metrics[metricName].With(labelValues).Set(m)
			e.metrics[lastPushTimeMetricName].With(labelValues).SetToCurrentTime()
			e.counterMetrics[countMetricName].With(labelValues).Inc()
		}
	}
}
