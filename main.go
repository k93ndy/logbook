package main

import (
    "flag"
    "os"
    "path/filepath"
    "encoding/json"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"

    log "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
)

type logConfig struct {
    Format string
    Out string
    Level string
}

type targetConfig struct {
    Namespace string
}

type authConfig struct {
    Mode string
    KubeConfig string
}

type config struct {
    Log logConfig
    Target targetConfig
    Auth authConfig
}

func initViper(conf *config) {
    //load configurations from file
    viper.SetConfigName("logbook")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("/etc/logbook/logbook")
    viper.AddConfigPath("$HOME/.logbook")
    viper.AddConfigPath(".")
    if err := viper.ReadInConfig(); err != nil {
        log.Infoln(err.Error())
    }

    //default values
    viper.SetDefault("target.namespace", "")
    viper.SetDefault("log.format", "json")
    viper.SetDefault("log.out", "stdout")
    viper.SetDefault("log.level", "debug")
    viper.SetDefault("auth.mode", "in-cluster")
    
    if err := viper.Unmarshal(&conf); err != nil {
        log.Errorf("unable to decode into struct, %v\n", err)
    }
}

func initLogrus(logConf *logConfig) {
    switch logConf.Format {
    case "json":
        log.SetFormatter(&log.JSONFormatter{})
    case "text":
        log.SetFormatter(&log.TextFormatter{})
    default:
        log.Errorf("log.format \"%v\" not supported, defaults to json.\n", logConf.Format)
        log.SetFormatter(&log.JSONFormatter{})
    }

    switch logConf.Out {
    case "stdout":
        log.SetOutput(os.Stdout)
    case "stderr":
        log.SetOutput(os.Stderr)
    default:
        log.Errorf("log.out \"%v\" not supported, defaults to stdout.", logConf.Out)
        log.SetOutput(os.Stdout)
    }

    switch logConf.Level {
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
        log.Errorf("log.level \"%v\" not supported, defaults to info.", logConf.Level)
        log.SetLevel(log.InfoLevel)
    }
}

//func initClientset() {
//}

func main() {
    var conf config
    initViper(&conf)
    initLogrus(&conf.Log)

    var kubeconfig *string
    if home := homeDir(); home != "" {
        kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
    } else {
        kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
    }
    flag.Parse()

    // use the current context in kubeconfig
    config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
    if err != nil {
        panic(err.Error())
    }

    // create the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        panic(err.Error())
    }

    eventWatchInterface, err := clientset.Events().Events("").Watch(metav1.ListOptions{})
    if err != nil {
        panic(err.Error())
    }
    log.Infoln("Watch interface created.")
    log.Infoln("Watch and log kubernetes events from now on.")

    for {
        select {
        case newEvent := <-eventWatchInterface.ResultChan():
             //marshalledEvent, err := json.MarshalIndent(newEvent, "", "    ")
             marshalledEvent, err := json.Marshal(newEvent)
             if err != nil {
                 panic(err.Error())
             }
             log.Infof("%s\n", string(marshalledEvent))
        }
    }

}

func homeDir() string {
    if h := os.Getenv("HOME"); h != "" {
        return h
    }
    return os.Getenv("USERPROFILE") // windows
}
