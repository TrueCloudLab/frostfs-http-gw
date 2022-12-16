package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/TrueCloudLab/frostfs-http-gw/downloader"
	"github.com/TrueCloudLab/frostfs-http-gw/metrics"
	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	"github.com/TrueCloudLab/frostfs-http-gw/response"
	"github.com/TrueCloudLab/frostfs-http-gw/uploader"
	"github.com/TrueCloudLab/frostfs-http-gw/utils"
	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
	"github.com/fasthttp/router"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type (
	app struct {
		log       *zap.Logger
		logLevel  zap.AtomicLevel
		pool      *pool.Pool
		owner     *user.ID
		cfg       *viper.Viper
		webServer *fasthttp.Server
		webDone   chan struct{}
		resolver  *resolver.ContainerResolver
		metrics   *gateMetrics
		services  []*metrics.Service
		settings  *appSettings
		servers   []Server
	}

	appSettings struct {
		Uploader   *uploader.Settings
		Downloader *downloader.Settings
	}

	// App is an interface for the main gateway function.
	App interface {
		Wait()
		Serve(context.Context)
	}

	// Option is an application option.
	Option func(a *app)

	gateMetrics struct {
		logger   *zap.Logger
		provider GateMetricsProvider
		mu       sync.RWMutex
		enabled  bool
	}

	GateMetricsProvider interface {
		SetHealth(int32)
		Unregister()
	}
)

// WithLogger returns Option to set a specific logger.
func WithLogger(l *zap.Logger, lvl zap.AtomicLevel) Option {
	return func(a *app) {
		if l == nil {
			return
		}
		a.log = l
		a.logLevel = lvl
	}
}

// WithConfig returns Option to use specific Viper configuration.
func WithConfig(c *viper.Viper) Option {
	return func(a *app) {
		if c == nil {
			return
		}
		a.cfg = c
	}
}

func newApp(ctx context.Context, opt ...Option) App {
	var (
		key *ecdsa.PrivateKey
		err error
	)

	a := &app{
		log:       zap.L(),
		cfg:       viper.GetViper(),
		webServer: new(fasthttp.Server),
		webDone:   make(chan struct{}),
	}
	for i := range opt {
		opt[i](a)
	}

	// -- setup FastHTTP server --
	a.webServer.Name = "frost-http-gw"
	a.webServer.ReadBufferSize = a.cfg.GetInt(cfgWebReadBufferSize)
	a.webServer.WriteBufferSize = a.cfg.GetInt(cfgWebWriteBufferSize)
	a.webServer.ReadTimeout = a.cfg.GetDuration(cfgWebReadTimeout)
	a.webServer.WriteTimeout = a.cfg.GetDuration(cfgWebWriteTimeout)
	a.webServer.DisableHeaderNamesNormalizing = true
	a.webServer.NoDefaultServerHeader = true
	a.webServer.NoDefaultContentType = true
	a.webServer.MaxRequestBodySize = a.cfg.GetInt(cfgWebMaxRequestBodySize)
	a.webServer.DisablePreParseMultipartForm = true
	a.webServer.StreamRequestBody = a.cfg.GetBool(cfgWebStreamRequestBody)
	// -- -- -- -- -- -- -- -- -- -- -- -- -- --
	key, err = getNeoFSKey(a)
	if err != nil {
		a.log.Fatal("failed to get neofs credentials", zap.Error(err))
	}

	var owner user.ID
	user.IDFromKey(&owner, key.PublicKey)
	a.owner = &owner

	var prm pool.InitParameters
	prm.SetKey(key)
	prm.SetNodeDialTimeout(a.cfg.GetDuration(cfgConTimeout))
	prm.SetNodeStreamTimeout(a.cfg.GetDuration(cfgStreamTimeout))
	prm.SetHealthcheckTimeout(a.cfg.GetDuration(cfgReqTimeout))
	prm.SetClientRebalanceInterval(a.cfg.GetDuration(cfgRebalance))
	prm.SetErrorThreshold(a.cfg.GetUint32(cfgPoolErrorThreshold))

	for i := 0; ; i++ {
		address := a.cfg.GetString(cfgPeers + "." + strconv.Itoa(i) + ".address")
		weight := a.cfg.GetFloat64(cfgPeers + "." + strconv.Itoa(i) + ".weight")
		priority := a.cfg.GetInt(cfgPeers + "." + strconv.Itoa(i) + ".priority")
		if address == "" {
			break
		}
		if weight <= 0 { // unspecified or wrong
			weight = 1
		}
		if priority <= 0 { // unspecified or wrong
			priority = 1
		}
		prm.AddNode(pool.NewNodeParam(priority, address, weight))
		a.log.Info("add connection", zap.String("address", address),
			zap.Float64("weight", weight), zap.Int("priority", priority))
	}

	a.pool, err = pool.NewPool(prm)
	if err != nil {
		a.log.Fatal("failed to create connection pool", zap.Error(err))
	}

	err = a.pool.Dial(ctx)
	if err != nil {
		a.log.Fatal("failed to dial pool", zap.Error(err))
	}

	a.initAppSettings()
	a.initResolver()
	a.initMetrics()

	return a
}

func (a *app) initAppSettings() {
	a.settings = &appSettings{
		Uploader:   &uploader.Settings{},
		Downloader: &downloader.Settings{},
	}

	a.updateSettings()
}

func (a *app) initResolver() {
	var err error
	a.resolver, err = resolver.NewContainerResolver(a.getResolverConfig())
	if err != nil {
		a.log.Fatal("failed to create resolver", zap.Error(err))
	}
}

func (a *app) getResolverConfig() ([]string, *resolver.Config) {
	resolveCfg := &resolver.Config{
		NeoFS:      resolver.NewNeoFSResolver(a.pool),
		RPCAddress: a.cfg.GetString(cfgRPCEndpoint),
	}

	order := a.cfg.GetStringSlice(cfgResolveOrder)
	if resolveCfg.RPCAddress == "" {
		order = remove(order, resolver.NNSResolver)
		a.log.Warn(fmt.Sprintf("resolver '%s' won't be used since '%s' isn't provided", resolver.NNSResolver, cfgRPCEndpoint))
	}

	if len(order) == 0 {
		a.log.Info("container resolver will be disabled because of resolvers 'resolver_order' is empty")
	}

	return order, resolveCfg
}

func (a *app) initMetrics() {
	gateMetricsProvider := metrics.NewGateMetrics(a.pool)
	a.metrics = newGateMetrics(a.log, gateMetricsProvider, a.cfg.GetBool(cfgPrometheusEnabled))
}

func newGateMetrics(logger *zap.Logger, provider GateMetricsProvider, enabled bool) *gateMetrics {
	if !enabled {
		logger.Warn("metrics are disabled")
	}
	return &gateMetrics{
		logger:   logger,
		provider: provider,
	}
}

func (m *gateMetrics) SetEnabled(enabled bool) {
	if !enabled {
		m.logger.Warn("metrics are disabled")
	}

	m.mu.Lock()
	m.enabled = enabled
	m.mu.Unlock()
}

func (m *gateMetrics) SetHealth(status int32) {
	m.mu.RLock()
	if !m.enabled {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	m.provider.SetHealth(status)
}

func (m *gateMetrics) Shutdown() {
	m.mu.Lock()
	if m.enabled {
		m.provider.SetHealth(0)
		m.enabled = false
	}
	m.provider.Unregister()
	m.mu.Unlock()
}

func remove(list []string, element string) []string {
	for i, item := range list {
		if item == element {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func getNeoFSKey(a *app) (*ecdsa.PrivateKey, error) {
	walletPath := a.cfg.GetString(cfgWalletPath)

	if len(walletPath) == 0 {
		a.log.Info("no wallet path specified, creating ephemeral key automatically for this run")
		key, err := keys.NewPrivateKey()
		if err != nil {
			return nil, err
		}
		return &key.PrivateKey, nil
	}
	w, err := wallet.NewWalletFromFile(walletPath)
	if err != nil {
		return nil, err
	}

	var password *string
	if a.cfg.IsSet(cfgWalletPassphrase) {
		pwd := a.cfg.GetString(cfgWalletPassphrase)
		password = &pwd
	}

	address := a.cfg.GetString(cfgWalletAddress)

	return getKeyFromWallet(w, address, password)
}

func getKeyFromWallet(w *wallet.Wallet, addrStr string, password *string) (*ecdsa.PrivateKey, error) {
	var addr util.Uint160
	var err error

	if addrStr == "" {
		addr = w.GetChangeAddress()
	} else {
		addr, err = flags.ParseAddress(addrStr)
		if err != nil {
			return nil, fmt.Errorf("invalid address")
		}
	}

	acc := w.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("couldn't find wallet account for %s", addrStr)
	}

	if password == nil {
		pwd, err := input.ReadPassword("Enter password > ")
		if err != nil {
			return nil, fmt.Errorf("couldn't read password")
		}
		password = &pwd
	}

	if err := acc.Decrypt(*password, w.Scrypt); err != nil {
		return nil, fmt.Errorf("couldn't decrypt account: %w", err)
	}

	return &acc.PrivateKey().PrivateKey, nil
}

func (a *app) Wait() {
	a.log.Info("starting application", zap.String("app_name", "frostfs-http-gw"), zap.String("version", Version))

	a.setHealthStatus()

	<-a.webDone // wait for web-server to be stopped
}

func (a *app) setHealthStatus() {
	a.metrics.SetHealth(1)
}

func (a *app) Serve(ctx context.Context) {
	uploadRoutes := uploader.New(ctx, a.AppParams(), a.settings.Uploader)
	downloadRoutes := downloader.New(ctx, a.AppParams(), a.settings.Downloader)

	// Configure router.
	a.configureRouter(uploadRoutes, downloadRoutes)

	a.startServices()
	a.initServers(ctx)

	for i := range a.servers {
		go func(i int) {
			a.log.Info("starting server", zap.String("address", a.servers[i].Address()))
			if err := a.webServer.Serve(a.servers[i].Listener()); err != nil && err != http.ErrServerClosed {
				a.log.Fatal("listen and serve", zap.Error(err))
			}
		}(i)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)

LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		case <-sigs:
			a.configReload()
		}
	}

	a.log.Info("shutting down web server", zap.Error(a.webServer.Shutdown()))

	a.metrics.Shutdown()
	a.stopServices()

	close(a.webDone)
}

func (a *app) configReload() {
	a.log.Info("SIGHUP config reload started")
	if !a.cfg.IsSet(cmdConfig) {
		a.log.Warn("failed to reload config because it's missed")
		return
	}
	if err := readConfig(a.cfg); err != nil {
		a.log.Warn("failed to reload config", zap.Error(err))
		return
	}
	if lvl, err := getLogLevel(a.cfg); err != nil {
		a.log.Warn("log level won't be updated", zap.Error(err))
	} else {
		a.logLevel.SetLevel(lvl)
	}

	if err := a.resolver.UpdateResolvers(a.getResolverConfig()); err != nil {
		a.log.Warn("failed to update resolvers", zap.Error(err))
	}

	if err := a.updateServers(); err != nil {
		a.log.Warn("failed to reload server parameters", zap.Error(err))
	}

	a.stopServices()
	a.startServices()

	a.updateSettings()

	a.metrics.SetEnabled(a.cfg.GetBool(cfgPrometheusEnabled))
	a.setHealthStatus()

	a.log.Info("SIGHUP config reload completed")
}

func (a *app) updateSettings() {
	a.settings.Uploader.SetDefaultTimestamp(a.cfg.GetBool(cfgUploaderHeaderEnableDefaultTimestamp))
	a.settings.Downloader.SetZipCompression(a.cfg.GetBool(cfgZipCompression))
}

func (a *app) startServices() {
	pprofConfig := metrics.Config{Enabled: a.cfg.GetBool(cfgPprofEnabled), Address: a.cfg.GetString(cfgPprofAddress)}
	pprofService := metrics.NewPprofService(a.log, pprofConfig)
	a.services = append(a.services, pprofService)
	go pprofService.Start()

	prometheusConfig := metrics.Config{Enabled: a.cfg.GetBool(cfgPrometheusEnabled), Address: a.cfg.GetString(cfgPrometheusAddress)}
	prometheusService := metrics.NewPrometheusService(a.log, prometheusConfig)
	a.services = append(a.services, prometheusService)
	go prometheusService.Start()
}

func (a *app) stopServices() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	for _, svc := range a.services {
		svc.ShutDown(ctx)
	}
}

func (a *app) configureRouter(uploadRoutes *uploader.Uploader, downloadRoutes *downloader.Downloader) {
	r := router.New()
	r.RedirectTrailingSlash = true
	r.NotFound = func(r *fasthttp.RequestCtx) {
		response.Error(r, "Not found", fasthttp.StatusNotFound)
	}
	r.MethodNotAllowed = func(r *fasthttp.RequestCtx) {
		response.Error(r, "Method Not Allowed", fasthttp.StatusMethodNotAllowed)
	}
	r.POST("/upload/{cid}", a.logger(uploadRoutes.Upload))
	a.log.Info("added path /upload/{cid}")
	r.GET("/get/{cid}/{oid}", a.logger(downloadRoutes.DownloadByAddress))
	r.HEAD("/get/{cid}/{oid}", a.logger(downloadRoutes.HeadByAddress))
	a.log.Info("added path /get/{cid}/{oid}")
	r.GET("/get_by_attribute/{cid}/{attr_key}/{attr_val:*}", a.logger(downloadRoutes.DownloadByAttribute))
	r.HEAD("/get_by_attribute/{cid}/{attr_key}/{attr_val:*}", a.logger(downloadRoutes.HeadByAttribute))
	a.log.Info("added path /get_by_attribute/{cid}/{attr_key}/{attr_val:*}")
	r.GET("/zip/{cid}/{prefix:*}", a.logger(downloadRoutes.DownloadZipped))
	a.log.Info("added path /zip/{cid}/{prefix}")

	a.webServer.Handler = r.Handler
}

func (a *app) logger(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		a.log.Info("request", zap.String("remote", ctx.RemoteAddr().String()),
			zap.ByteString("method", ctx.Method()),
			zap.ByteString("path", ctx.Path()),
			zap.ByteString("query", ctx.QueryArgs().QueryString()),
			zap.Uint64("id", ctx.ID()))
		h(ctx)
	}
}

func (a *app) AppParams() *utils.AppParams {
	return &utils.AppParams{
		Logger:   a.log,
		Pool:     a.pool,
		Owner:    a.owner,
		Resolver: a.resolver,
	}
}

func (a *app) initServers(ctx context.Context) {
	serversInfo := fetchServers(a.cfg)

	a.servers = make([]Server, len(serversInfo))
	for i, serverInfo := range serversInfo {
		a.log.Info("added server",
			zap.String("address", serverInfo.Address), zap.Bool("tls enabled", serverInfo.TLS.Enabled),
			zap.String("tls cert", serverInfo.TLS.CertFile), zap.String("tls key", serverInfo.TLS.KeyFile))
		a.servers[i] = newServer(ctx, serverInfo, a.log)
	}
}

func (a *app) updateServers() error {
	serversInfo := fetchServers(a.cfg)

	if len(serversInfo) != len(a.servers) {
		return fmt.Errorf("invalid servers configuration: length mismatch: old '%d', new '%d", len(a.servers), len(serversInfo))
	}

	for i, serverInfo := range serversInfo {
		if serverInfo.Address != a.servers[i].Address() {
			return fmt.Errorf("invalid servers configuration: addresses mismatch: old '%s', new '%s", a.servers[i].Address(), serverInfo.Address)
		}

		if serverInfo.TLS.Enabled {
			if err := a.servers[i].UpdateCert(serverInfo.TLS.CertFile, serverInfo.TLS.KeyFile); err != nil {
				return fmt.Errorf("failed to update tls certs: %w", err)
			}
		}
	}

	return nil
}
