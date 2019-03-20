package main

import (
    //"flag"
    "os"
    "os/signal"
    "syscall"
    "path/filepath"
    "encoding/json"
    //"reflect"
    "fmt"

    "k8s.io/apimachinery/pkg/watch"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/rest"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
    "github.com/spf13/viper"

    "github.com/k93ndy/logbook/cmd"
    "github.com/k93ndy/logbook/config"
)

func initConfig() (*config.Config, error) {
    //default values
    viper.SetDefault("target.namespace", "")
    viper.SetDefault("target.listoptions.timeoutseconds", "0")
    viper.SetDefault("log.format", "json")
    viper.SetDefault("log.out", "stdout")
    viper.SetDefault("log.filename", "k8s-events.log")
    viper.SetDefault("log.level", "info")
    viper.SetDefault("auth.mode", "in-cluster")
 
    // cooperate with cobra, command flags have a higher priority
    cmd.Execute()

    //load configurations from file
    if viper.Get("config") != nil {
        viper.SetConfigFile(viper.Get("config").(string))
    } else {
        viper.SetConfigName("logbook")
        viper.SetConfigType("yaml")
        viper.AddConfigPath("/etc/logbook/logbook")
        viper.AddConfigPath("$HOME/.logbook")
        viper.AddConfigPath(".")
    }
    if err := viper.ReadInConfig(); err != nil {
        log.Infoln(err.Error())
    }

    var cfg config.Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, errors.Wrap(err, "Error occured during unmarshal config")
    }
    
    return &cfg, nil

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

func createClientset(authCfg *config.AuthConfig) (*kubernetes.Clientset, error){
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
        log.Panicf("auth.mode \"%v\" not supported. Logbook will be terminated.\n", authCfg.Mode)
    }

    // create clientset
    return kubernetes.NewForConfig(config)
}

func main() {
    // register signals
    sigChan := make(chan os.Signal, 1)
    signal.Ignore()
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

    // initiate configuration
    log.SetFormatter(&log.JSONFormatter{})
    cfg, err := initConfig()
    if err != nil {
        panic(err.Error())
    }
    log.Infof("Initialized with configuration: %+v\n", cfg)

    // initiate logging
    logFile, err := initLogrus(&cfg.Log)
    if err != nil {
        panic(err.Error())
    }
    
    // handle signals
    go func (logFile *os.File) {
        log.Infoln(logFile)
        for {
            select {
            case sig := <-sigChan:
                switch sig {
                case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
                    log.Infof("Signal %v received. Logbook will be shutdown.", sig)
                    if logFile != nil {
                        //if err := logFile.Sync(); err != nil {
                        //    log.Infoln(err)
                        //}
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
}

func homeDir() string {
    if h := os.Getenv("HOME"); h != "" {
        return h
    }
    return os.Getenv("USERPROFILE") // windows
}
