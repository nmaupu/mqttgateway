package conf

type ConfigMqttTopic struct {
	Topic         string               `yaml:"topic"`
	GjsonPatterns []ConfigGjsonPattern `mapstructure:"patterns"`
}

type ConfigGjsonPattern struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
}

type MqttGatewayConfig struct {
	Topics []ConfigMqttTopic `mapstructure:"topics"`
}

func (c ConfigMqttTopic) GetAllPatterns() []ConfigGjsonPattern {
	ret := make([]ConfigGjsonPattern, 0)
	for _, v := range c.GjsonPatterns {
		ret = append(ret, v)
	}
	return ret
}

func (c MqttGatewayConfig) GetTopic(topic string) *ConfigMqttTopic {
	for _, v := range c.Topics {
		if topic == v.Topic {
			return &v
		}
	}
	return nil
}
