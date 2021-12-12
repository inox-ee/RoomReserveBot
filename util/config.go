package util

import "github.com/spf13/viper"

type Config struct {
	SlackSigningSecret string `mapstructure:"SLACK_SIGNING_SECRET"`
	SlackBotToken      string `mapstructure:"SLACK_BOT_TOKEN"`
	SlackSocketToken   string `mapstructure:"SLACK_SOCKET_TOKEN"`
}

func LoadConfig(path, appName string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(appName)
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return
}
