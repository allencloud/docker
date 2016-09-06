package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/opts"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/pflag"
)

const (
	// DefaultTrustKeyFile is the default filename for the trust key
	DefaultTrustKeyFile = "key.json"
	// DefaultCaFile is the default filename for the CA pem file
	DefaultCaFile = "ca.pem"
	// DefaultKeyFile is the default filename for the key pem file
	DefaultKeyFile = "key.pem"
	// DefaultCertFile is the default filename for the cert pem file
	DefaultCertFile = "cert.pem"
	// FlagTLSVerify is the flag name for the tls verification option
	FlagTLSVerify = "tlsverify"
)

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTLSVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

// CommonOptions are options common to both the client and the daemon.
type CommonOptions struct {
	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	TrustKey   string
}

// NewCommonOptions returns a new CommonOptions
func NewCommonOptions() *CommonOptions {
	return &CommonOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (commonOpts *CommonOptions) InstallFlags(flags *pflag.FlagSet) {
	if dockerCertPath == "" {
		dockerCertPath = cliconfig.ConfigDir()
	}

	flags.BoolVarP(&commonOpts.Debug, "debug", "D", false, "启用调试模式")
	flags.StringVarP(&commonOpts.LogLevel, "log-level", "l", "info", "设置日志级别")
	flags.BoolVar(&commonOpts.TLS, "tls", false, "使用 TLS; 可以通过 --tlsverify 参数制定")
	flags.BoolVar(&commonOpts.TLSVerify, FlagTLSVerify, dockerTLSVerify, "使用 TLS 来验证远程连接")

	// TODO use flag flags.String("identity"}, "i", "", "Path to libtrust key file")

	commonOpts.TLSOptions = &tlsconfig.Options{}
	tlsOptions := commonOpts.TLSOptions
	flags.StringVar(&tlsOptions.CAFile, "tlscacert", filepath.Join(dockerCertPath, DefaultCaFile), "只通过此CA受信的cert")
	flags.StringVar(&tlsOptions.CertFile, "tlscert", filepath.Join(dockerCertPath, DefaultCertFile), "TLS 证书文件路径")
	flags.StringVar(&tlsOptions.KeyFile, "tlskey", filepath.Join(dockerCertPath, DefaultKeyFile), "TLS 密钥文件路径")

	hostOpt := opts.NewNamedListOptsRef("hosts", &commonOpts.Hosts, opts.ValidateHost)
	flags.VarP(hostOpt, "host", "H", "Docker引擎监听的套接字")
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (commonOpts *CommonOptions) SetDefaultOptions(flags *pflag.FlagSet) {
	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on tls
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if flags.Changed(FlagTLSVerify) || commonOpts.TLSVerify {
		commonOpts.TLS = true
	}

	if !commonOpts.TLS {
		commonOpts.TLSOptions = nil
	} else {
		tlsOptions := commonOpts.TLSOptions
		tlsOptions.InsecureSkipVerify = !commonOpts.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !flags.Changed("tlscert") {
			if _, err := os.Stat(tlsOptions.CertFile); os.IsNotExist(err) {
				tlsOptions.CertFile = ""
			}
		}
		if !flags.Changed("tlskey") {
			if _, err := os.Stat(tlsOptions.KeyFile); os.IsNotExist(err) {
				tlsOptions.KeyFile = ""
			}
		}
	}
}

// SetDaemonLogLevel sets the logrus logging level
// TODO: this is a bad name, it applies to the client as well.
func SetDaemonLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "不能解析日志级别: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
