package config

type LogConfig struct {
    Format string
    Out string
    Level string
}

type TargetConfig struct {
    Namespace string
}

type AuthConfig struct {
    Mode string
    KubeConfig string
}

type Config struct {
    Log LogConfig
    Target TargetConfig
    Auth AuthConfig
}
