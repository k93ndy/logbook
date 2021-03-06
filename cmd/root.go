// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	//homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/k93ndy/logbook/config"
)

var cfgFile string
var cfg config.Config
var logFile *os.File

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "logbook",
	Short: "Logbook is a kubernetes event logger.",
	Long: `Logbook is a kubernetes event logger which can be used
both in-cluster(use kubernetes ServiceAccount for auth) 
and out-of-cluster(use kubeconfig file for auth).`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		// register signals
		sigChan := make(chan os.Signal, 1)
		signal.Ignore()
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		// handle signals
		go func(logFile *os.File) {
			log.Infoln(logFile)
			for {
				select {
				case sig := <-sigChan:
					switch sig {
					case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
						log.Infof("Signal %v received. Logbook will be shutdown.", sig)
						if logFile != nil {
							if err := logFile.Sync(); err != nil {
								fmt.Println(err)
							}
							if err := logFile.Close(); err != nil {
								fmt.Println(err)
							}
							fmt.Println("Log flushed.")
						}
						os.Exit(0)
					}
				}
			}
		}(logFile)

		// create clientset
		clientset, err := createClientset(&cfg.Auth)
		if err != nil {
			panic(err.Error())
		}

		// create watch interface
		eventWatcher, err := clientset.CoreV1().Events(cfg.Target.Namespace).Watch(cfg.Target.ListOptions)
		if err != nil {
			panic(err.Error())
		}
		log.Infoln("Watcher was successfully created. Kubernetes events will be logged from now on.")
		for {
			select {
			case event, ok := <-eventWatcher.ResultChan():
				if !ok {
					log.Warnln("Watcher timed out.")
					os.Exit(0)
				}
				switch event.Type {
				case watch.Modified, watch.Added, watch.Error:
					//marshalledEvent, err := json.MarshalIndent(event, "", "    ")
					marshalledEvent, err := json.Marshal(event)
					if err != nil {
						panic(err.Error())
					}
					log.Infof("%s\n", string(marshalledEvent))
				}
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $PWD/logbook.yaml)")
	rootCmd.PersistentFlags().StringVar(&cfg.Auth.Mode, "mode", "", "running mode (default is in-cluster mode)")
	rootCmd.PersistentFlags().StringVar(&cfg.Auth.KubeConfig, "kubeconfig", "", "absolute path of kubeconfig file (default is $HOME/.kube/config, only used in out-of-cluster mode)")
	rootCmd.PersistentFlags().StringVar(&cfg.Target.Namespace, "namespace", "", "namespace to watch (default is all namespaces)")
	rootCmd.PersistentFlags().StringVar(&cfg.Log.Format, "log-format", "", "log format (default is json)")
	rootCmd.PersistentFlags().StringVar(&cfg.Log.Out, "log-out", "", "log output (default is stdout)")
	rootCmd.PersistentFlags().StringVar(&cfg.Log.Level, "log-level", "", "log level (default is info)")
	rootCmd.PersistentFlags().StringVar(&cfg.Log.Filename, "log-filename", "", "full path of log file with filename (valid only when log-out is set to file. default is k8s-events.log in the same directory with logbook)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// bind flags with viper config
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("auth.mode", rootCmd.PersistentFlags().Lookup("mode"))
	viper.BindPFlag("auth.kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	viper.BindPFlag("target.namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
	viper.BindPFlag("log.out", rootCmd.PersistentFlags().Lookup("log-out"))
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("log.filename", rootCmd.PersistentFlags().Lookup("log-filename"))

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match

	// default values
	viper.SetDefault("target.namespace", "")
	// 5 years
	timeoutSeconds := int64(157680000)
	viper.SetDefault("target.listoptions.timeoutseconds", &timeoutSeconds)
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.out", "stdout")
	viper.SetDefault("log.filename", "k8s-events.log")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("auth.mode", "in-cluster")

	// load configurations from file
	if viper.Get("config") != nil && viper.Get("config").(string) != "" {
		viper.SetConfigFile(viper.Get("config").(string))
	} else {
		viper.SetConfigName("logbook")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/etc/logbook/logbook")
		viper.AddConfigPath("$HOME/.logbook")
		viper.AddConfigPath(".")
	}
	if err := viper.ReadInConfig(); err == nil {
		log.Infoln("Using config file:", viper.ConfigFileUsed())
	} else {
		log.Infoln(err.Error())
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		log.Panicln(errors.Wrap(err, "Error occured during unmarshal config"))
	}
	log.Infof("Initialized with configuration: %+v\n", cfg)

	//init logrus
	var err error
	if logFile, err = initLogrus(&cfg.Log); err != nil {
		panic(err.Error())
	}
}

func initLogrus(logCfg *config.LogConfig) (*os.File, error) {
	var file *os.File
	switch logCfg.Format {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{})
	default:
		log.Errorf("log.format \"%v\" not supported, defaults to json.\n", logCfg.Format)
		log.SetFormatter(&log.JSONFormatter{})
	}

	switch logCfg.Out {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "file":
		f, err := os.OpenFile(logCfg.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, errors.Wrap(err, "Error occured during open log file")
		}
		log.SetOutput(f)
		file = f
	default:
		log.Errorf("log.out \"%v\" not supported, defaults to stdout.", logCfg.Out)
		log.SetOutput(os.Stdout)
	}

	switch logCfg.Level {
	case "panic":
		log.SetLevel(log.PanicLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "trace":
		log.SetLevel(log.TraceLevel)
	default:
		log.Errorf("log.level \"%v\" not supported, defaults to info.", logCfg.Level)
		log.SetLevel(log.InfoLevel)
	}

	return file, nil
}

func createClientset(authCfg *config.AuthConfig) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	switch authCfg.Mode {
	case "in-cluster":
		log.Infoln("Logbook will start in in-cluster mode.")
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "Error occured during start as in-cluster mode")
		}
	case "out-of-cluster":
		log.Infoln("Logbook will start in out-of-cluster mode.")
		if authCfg.KubeConfig == "" {
			log.Infoln("kubeconfig not provided. Will use kubeconfig file in default path.")
			if home := homeDir(); home != "" {
				authCfg.KubeConfig = filepath.Join(home, ".kube", "config")
			}
		}
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", authCfg.KubeConfig)
		if err != nil {
			panic(err.Error())
		}
	default:
		err := fmt.Errorf("auth.mode \"%v\" not supported. Logbook will be terminated.\n", authCfg.Mode)
		return nil, errors.Wrap(err, "Error occured during creating clientset")
	}

	return kubernetes.NewForConfig(config)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
