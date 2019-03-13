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
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

    "github.com/k93ndy/logbook/config"
)

var cfgFile string
var cfg config.Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "logbook",
	Short: "Logbook is a kubernetes event logger.",
	Long: `Logbook is a kubernetes event logger which can be used
both in-cluster(use kubernetes ServiceAccount for auth) 
and out-of-cluster(use kubeconfig file for auth).`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
    //Run: func(cmd *cobra.Command, args []string) {
    //},
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
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".test" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("logbook")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
