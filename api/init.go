package api

import (
	"github.com/spf13/viper"
)

func init() {
	viper.AddConfigPath("/var/run/config/naisd.io")
}

func in() {
	Spec
}
