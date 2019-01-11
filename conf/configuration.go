package conf

import (
	"github.com/opsgenie/marid2/git"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LocalSourceType = "local"
	GitSourceType   = "git"
)

type Configuration struct {
	ApiKey 			string 			`json:"apiKey" yaml:"apiKey"`
	BaseUrl 		string			`json:"baseUrl" yaml:"baseUrl"`
	ActionMappings 	ActionMappings 	`json:"actionMappings" yaml:"actionMappings"`
	PollerConf 		PollerConf 		`json:"pollerConf" yaml:"pollerConf"`
	PoolConf 		PoolConf 		`json:"poolConf" yaml:"poolConf"`
	LogLevel		logrus.Level	`json:"logLevel" yaml:"logLevel"`
}

type ActionName string

type ActionMappings map[ActionName]MappedAction

func (m *ActionMappings) GitActions() []git.GitOptions {

	opts := make([]git.GitOptions, 0)
	for _, action := range *m {
		if (action.GitOptions != git.GitOptions{}) {
			opts = append(opts, action.GitOptions)
		}
	}

	return opts
}

type MappedAction struct {
	SourceType           string         `json:"sourceType" yaml:"sourceType"`
	GitOptions           git.GitOptions `json:"gitOptions" yaml:"gitOptions"`
	Filepath             string         `json:"filepath" yaml:"filepath"`
	EnvironmentVariables []string       `json:"environmentVariables" yaml:"environmentVariables"`
}

type PollerConf struct {
	PollingWaitIntervalInMillis time.Duration `json:"pollingWaitIntervalInMillis" yaml:"pollingWaitIntervalInMillis"`
	VisibilityTimeoutInSeconds  int64         `json:"visibilityTimeoutInSeconds" yaml:"visibilityTimeoutInSeconds"`
	MaxNumberOfMessages         int64         `json:"maxNumberOfMessages" yaml:"maxNumberOfMessages"`
}

type PoolConf struct {
	MaxNumberOfWorker        int32			`json:"maxNumberOfWorker" yaml:"maxNumberOfWorker"`
	MinNumberOfWorker        int32			`json:"minNumberOfWorker" yaml:"minNumberOfWorker"`
	QueueSize                int32			`json:"queueSize" yaml:"queueSize"`
	KeepAliveTimeInMillis    time.Duration	`json:"keepAliveTimeInMillis" yaml:"keepAliveTimeInMillis"`
	MonitoringPeriodInMillis time.Duration	`json:"monitoringPeriodInMillis" yaml:"monitoringPeriodInMillis"`
}

var readConfigurationFromGitFunc = readConfigurationFromGit
var readConfigurationFromLocalFunc = readConfigurationFromLocal

var defaultConfFilepath = filepath.Join("~","opsgenie", "maridConfig.json")

func ReadConfFile() (*Configuration, error) {

	confSource := os.Getenv("MARID_CONF_SOURCE")
	conf, err := readConfFileFromSource(strings.ToLower(confSource))
	if err != nil {
		return nil, err
	}

	err = validateConfiguration(conf)
	if err != nil {
		return nil, err
	}

	addHomeDirPrefixToLocalActionFilepaths(&conf.ActionMappings)
	chmodLocalActions(&conf.ActionMappings, 0700)
	addHomeDirPrefixToPrivateKeyFilepaths(&conf.ActionMappings)

	return conf, nil
}

func validateConfiguration(conf *Configuration) error {

	if conf == nil || conf == (&Configuration{}) {
		return errors.New("The configuration is empty.")
	}
	if len(conf.ActionMappings) == 0 {
		return errors.New("Action mappings configuration is not found in the configuration file.")
	}
	if conf.ApiKey == "" {
		return errors.New("ApiKey is not found in the configuration file.")
	}
	if conf.BaseUrl == "" {
		return errors.New("BaseUrl is not found in the configuration file.")
	}
	return nil
}

func readConfFileFromSource(confSource string) (*Configuration, error) {

	switch confSource {
	case GitSourceType:
		url := os.Getenv("MARID_CONF_GIT_URL")
		privateKeyFilepath := os.Getenv("MARID_CONF_GIT_PRIVATE_KEY_FILEPATH")
		passphrase := os.Getenv("MARID_CONF_GIT_PASSPHRASE")
		confFilepath := os.Getenv("MARID_CONF_GIT_FILEPATH")

		if privateKeyFilepath != "" {
			privateKeyFilepath = addHomeDirPrefix(privateKeyFilepath)
		}

		if confFilepath == "" {
			return nil, errors.New("Git configuration filepath could not be empty.")
		}

		return readConfigurationFromGitFunc(url, privateKeyFilepath, passphrase, confFilepath)
	case LocalSourceType:
		confFilepath := os.Getenv("MARID_CONF_LOCAL_FILEPATH")

		if len(confFilepath) <= 0 {
			confFilepath = addHomeDirPrefix(defaultConfFilepath) // todo when packaging, decide default confPath. homeDir can exist or not according to whether user.Current() exist or not. If there is only root user, homeDir will be "/".
		} else {
			confFilepath = addHomeDirPrefix(confFilepath)
		}

		return readConfigurationFromLocalFunc(confFilepath)
	default:
		return nil, errors.Errorf("Unknown configuration source [%s].", confSource)
	}
}
