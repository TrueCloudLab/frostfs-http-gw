package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	defaultRebalanceTimer = 60 * time.Second
	defaultRequestTimeout = 15 * time.Second
	defaultConnectTimeout = 10 * time.Second
	defaultStreamTimeout  = 10 * time.Second

	defaultShutdownTimeout = 15 * time.Second

	defaultPoolErrorThreshold uint32 = 100

	cfgServer      = "server"
	cfgTLSEnabled  = "tls.enabled"
	cfgTLSCertFile = "tls.cert_file"
	cfgTLSKeyFile  = "tls.key_file"

	// Web.
	cfgWebReadBufferSize     = "web.read_buffer_size"
	cfgWebWriteBufferSize    = "web.write_buffer_size"
	cfgWebReadTimeout        = "web.read_timeout"
	cfgWebWriteTimeout       = "web.write_timeout"
	cfgWebStreamRequestBody  = "web.stream_request_body"
	cfgWebMaxRequestBodySize = "web.max_request_body_size"

	// Metrics / Profiler.
	cfgPrometheusEnabled = "prometheus.enabled"
	cfgPrometheusAddress = "prometheus.address"
	cfgPprofEnabled      = "pprof.enabled"
	cfgPprofAddress      = "pprof.address"

	// Pool config.
	cfgConTimeout         = "connect_timeout"
	cfgStreamTimeout      = "stream_timeout"
	cfgReqTimeout         = "request_timeout"
	cfgRebalance          = "rebalance_timer"
	cfgPoolErrorThreshold = "pool_error_threshold"

	// Logger.
	cfgLoggerLevel = "logger.level"

	// Wallet.
	cfgWalletPassphrase = "wallet.passphrase"
	cfgWalletPath       = "wallet.path"
	cfgWalletAddress    = "wallet.address"

	// Uploader Header.
	cfgUploaderHeaderEnableDefaultTimestamp = "upload_header.use_default_timestamp"

	// Peers.
	cfgPeers = "peers"

	// NeoGo.
	cfgRPCEndpoint = "rpc_endpoint"

	// Resolving.
	cfgResolveOrder = "resolve_order"

	// Zip compression.
	cfgZipCompression = "zip.compression"

	// Command line args.
	cmdHelp          = "help"
	cmdVersion       = "version"
	cmdPprof         = "pprof"
	cmdMetrics       = "metrics"
	cmdWallet        = "wallet"
	cmdAddress       = "address"
	cmdConfig        = "config"
	cmdListenAddress = "listen_address"
)

var ignore = map[string]struct{}{
	cfgPeers:   {},
	cmdHelp:    {},
	cmdVersion: {},
}

func settings() *viper.Viper {
	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvPrefix(Prefix)
	v.AllowEmptyEnv(true)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// flags setup:
	flags := pflag.NewFlagSet("commandline", pflag.ExitOnError)
	flags.SetOutput(os.Stdout)
	flags.SortFlags = false

	flags.Bool(cmdPprof, false, "enable pprof")
	flags.Bool(cmdMetrics, false, "enable prometheus")

	help := flags.BoolP(cmdHelp, "h", false, "show help")
	version := flags.BoolP(cmdVersion, "v", false, "show version")

	flags.StringP(cmdWallet, "w", "", `path to the wallet`)
	flags.String(cmdAddress, "", `address of wallet account`)
	flags.String(cmdConfig, "", "config path")
	flags.Duration(cfgConTimeout, defaultConnectTimeout, "gRPC connect timeout")
	flags.Duration(cfgStreamTimeout, defaultStreamTimeout, "gRPC individual message timeout")
	flags.Duration(cfgReqTimeout, defaultRequestTimeout, "gRPC request timeout")
	flags.Duration(cfgRebalance, defaultRebalanceTimer, "gRPC connection rebalance timer")

	flags.String(cmdListenAddress, "0.0.0.0:8080", "addresses to listen")
	flags.String(cfgTLSCertFile, "", "TLS certificate path")
	flags.String(cfgTLSKeyFile, "", "TLS key path")
	peers := flags.StringArrayP(cfgPeers, "p", nil, "FrostFS nodes")

	resolveMethods := flags.StringSlice(cfgResolveOrder, []string{resolver.NNSResolver, resolver.DNSResolver}, "set container name resolve order")

	// set defaults:

	// logger:
	v.SetDefault(cfgLoggerLevel, "debug")

	// pool:
	v.SetDefault(cfgPoolErrorThreshold, defaultPoolErrorThreshold)

	// web-server:
	v.SetDefault(cfgWebReadBufferSize, 4096)
	v.SetDefault(cfgWebWriteBufferSize, 4096)
	v.SetDefault(cfgWebReadTimeout, time.Minute*10)
	v.SetDefault(cfgWebWriteTimeout, time.Minute*5)
	v.SetDefault(cfgWebStreamRequestBody, true)
	v.SetDefault(cfgWebMaxRequestBodySize, fasthttp.DefaultMaxRequestBodySize)

	// upload header
	v.SetDefault(cfgUploaderHeaderEnableDefaultTimestamp, false)

	// zip:
	v.SetDefault(cfgZipCompression, false)

	// metrics
	v.SetDefault(cfgPprofAddress, "localhost:8083")
	v.SetDefault(cfgPrometheusAddress, "localhost:8084")

	// Binding flags
	if err := v.BindPFlag(cfgPprofEnabled, flags.Lookup(cmdPprof)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(cfgPrometheusEnabled, flags.Lookup(cmdMetrics)); err != nil {
		panic(err)
	}

	if err := v.BindPFlag(cfgWalletPath, flags.Lookup(cmdWallet)); err != nil {
		panic(err)
	}

	if err := v.BindPFlag(cfgWalletAddress, flags.Lookup(cmdAddress)); err != nil {
		panic(err)
	}

	if err := v.BindPFlags(flags); err != nil {
		panic(err)
	}

	if err := v.BindPFlag(cfgServer+".0.address", flags.Lookup(cmdListenAddress)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(cfgServer+".0."+cfgTLSKeyFile, flags.Lookup(cfgTLSKeyFile)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(cfgServer+".0."+cfgTLSCertFile, flags.Lookup(cfgTLSCertFile)); err != nil {
		panic(err)
	}

	if err := flags.Parse(os.Args); err != nil {
		panic(err)
	}

	if v.IsSet(cfgServer+".0."+cfgTLSKeyFile) && v.IsSet(cfgServer+".0."+cfgTLSCertFile) {
		v.Set(cfgServer+".0."+cfgTLSEnabled, true)
	}

	if resolveMethods != nil {
		v.SetDefault(cfgResolveOrder, *resolveMethods)
	}

	switch {
	case help != nil && *help:
		fmt.Printf("FrostFS HTTP Gateway %s\n", Version)
		flags.PrintDefaults()

		fmt.Println()
		fmt.Println("Default environments:")
		fmt.Println()
		keys := v.AllKeys()
		sort.Strings(keys)

		for i := range keys {
			if _, ok := ignore[keys[i]]; ok {
				continue
			}

			defaultValue := v.GetString(keys[i])
			if len(defaultValue) == 0 {
				continue
			}

			k := strings.Replace(keys[i], ".", "_", -1)
			fmt.Printf("%s_%s = %s\n", Prefix, strings.ToUpper(k), defaultValue)
		}

		fmt.Println()
		fmt.Println("Peers preset:")
		fmt.Println()

		fmt.Printf("%s_%s_[N]_ADDRESS = string\n", Prefix, strings.ToUpper(cfgPeers))
		fmt.Printf("%s_%s_[N]_WEIGHT = float\n", Prefix, strings.ToUpper(cfgPeers))

		os.Exit(0)
	case version != nil && *version:
		fmt.Printf("FrostFS HTTP Gateway\nVersion: %s\nGoVersion: %s\n", Version, runtime.Version())
		os.Exit(0)
	}

	if v.IsSet(cmdConfig) {
		if err := readConfig(v); err != nil {
			panic(err)
		}
	}

	if peers != nil && len(*peers) > 0 {
		for i := range *peers {
			v.SetDefault(cfgPeers+"."+strconv.Itoa(i)+".address", (*peers)[i])
			v.SetDefault(cfgPeers+"."+strconv.Itoa(i)+".weight", 1)
			v.SetDefault(cfgPeers+"."+strconv.Itoa(i)+".priority", 1)
		}
	}

	return v
}

func readConfig(v *viper.Viper) error {
	cfgFileName := v.GetString(cmdConfig)
	cfgFile, err := os.Open(cfgFileName)
	if err != nil {
		return err
	}
	if err = v.ReadConfig(cfgFile); err != nil {
		return err
	}

	return cfgFile.Close()
}

// newLogger constructs a zap.Logger instance for current application.
// Panics on failure.
//
// Logger is built from zap's production logging configuration with:
//   - parameterized level (debug by default)
//   - console encoding
//   - ISO8601 time encoding
//
// Logger records a stack trace for all messages at or above fatal level.
//
// See also zapcore.Level, zap.NewProductionConfig, zap.AddStacktrace.
func newLogger(v *viper.Viper) (*zap.Logger, zap.AtomicLevel) {
	lvl, err := getLogLevel(v)
	if err != nil {
		panic(err)
	}

	c := zap.NewProductionConfig()
	c.Level = zap.NewAtomicLevelAt(lvl)
	c.Encoding = "console"
	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	l, err := c.Build(
		zap.AddStacktrace(zap.NewAtomicLevelAt(zap.FatalLevel)),
	)
	if err != nil {
		panic(fmt.Sprintf("build zap logger instance: %v", err))
	}

	return l, c.Level
}

func getLogLevel(v *viper.Viper) (zapcore.Level, error) {
	var lvl zapcore.Level
	lvlStr := v.GetString(cfgLoggerLevel)
	err := lvl.UnmarshalText([]byte(lvlStr))
	if err != nil {
		return lvl, fmt.Errorf("incorrect logger level configuration %s (%v), "+
			"value should be one of %v", lvlStr, err, [...]zapcore.Level{
			zapcore.DebugLevel,
			zapcore.InfoLevel,
			zapcore.WarnLevel,
			zapcore.ErrorLevel,
			zapcore.DPanicLevel,
			zapcore.PanicLevel,
			zapcore.FatalLevel,
		})
	}
	return lvl, nil
}

func fetchServers(v *viper.Viper) []ServerInfo {
	var servers []ServerInfo

	for i := 0; ; i++ {
		key := cfgServer + "." + strconv.Itoa(i) + "."

		var serverInfo ServerInfo
		serverInfo.Address = v.GetString(key + "address")
		serverInfo.TLS.Enabled = v.GetBool(key + cfgTLSEnabled)
		serverInfo.TLS.KeyFile = v.GetString(key + cfgTLSKeyFile)
		serverInfo.TLS.CertFile = v.GetString(key + cfgTLSCertFile)

		if serverInfo.Address == "" {
			break
		}

		servers = append(servers, serverInfo)
	}

	return servers
}
