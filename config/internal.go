package config

import "github.com/ahmetson/handler-lib/config"

func ManagerName(url string) string {
	fileName := config.UrlToFileName(url)
	return "manager." + fileName
}
