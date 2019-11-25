package main

import (
	"fmt"
	"net/http"

	"github.com/nmaupu/mqttgateway/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/spf13/viper"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	Progname       = "mqttgateway"
	ConfigFileName = "mqttgateway-config"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9337").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	brokerAddress = kingpin.Flag("mqtt.broker-address", "Address of the MQTT broker.").Envar("MQTT_BROKER_ADDRESS").Default("tcp://localhost:1883").String()
	topic         = kingpin.Flag("mqtt.topic", "MQTT topic to subscribe to").Envar("MQTT_TOPIC").Default("#").String()
	prefix        = kingpin.Flag("mqtt.prefix", "MQTT topic prefix to remove when creating metrics").Envar("MQTT_PREFIX").Default("").String()
	username      = kingpin.Flag("mqtt.username", "MQTT username").Envar("MQTT_USERNAME").String()
	password      = kingpin.Flag("mqtt.password", "MQTT password").Envar("MQTT_PASSWORD").String()
	clientID      = kingpin.Flag("mqtt.clientid", "MQTT client ID").Envar("MQTT_CLIENT_ID").String()
	Config        conf.MqttGatewayConfig
)

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Parse()

	// Viper - config file
	viper.SetConfigName(ConfigFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", Progname))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s/", Progname))
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Unable to read config file: %+v\n", err)
	}

	err = viper.Unmarshal(&Config)
	if err != nil {
		log.Fatalf("Error unmarshaling config file: %+v\n", err)
	}

	log.Debugf("Config content = %+v\n", Config)

	prometheus.MustRegister(newMQTTExporter())

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "UP")
	})
	log.Infoln("Listening on", *listenAddress)
	err = http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}
