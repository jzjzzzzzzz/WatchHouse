package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"watchhouse/internal/authz"
	"watchhouse/internal/controlstore"
	"watchhouse/internal/detection"
	"watchhouse/internal/secretfile"
	"watchhouse/internal/transport"
)

type options struct{ listen, databaseURLFile, rolesFile, ca, certificate, key, serverName string }

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

func run(args []string, errOut *os.File) int {
	flags := flag.NewFlagSet("watchhouse-control", flag.ContinueOnError)
	flags.SetOutput(errOut)
	var config options
	flags.StringVar(&config.listen, "listen", "127.0.0.1:8443", "literal IP and TCP port")
	flags.StringVar(&config.databaseURLFile, "database-url-file", "", "private file containing PostgreSQL URL")
	flags.StringVar(&config.rolesFile, "roles-file", "", "root-managed human role map")
	flags.StringVar(&config.ca, "client-ca", "", "private agent CA bundle")
	flags.StringVar(&config.certificate, "tls-cert", "", "control server certificate")
	flags.StringVar(&config.key, "tls-key", "", "control server private key")
	flags.StringVar(&config.serverName, "server-name", "", "DNS name asserted by control certificate")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || config.databaseURLFile == "" || config.rolesFile == "" || config.ca == "" || config.certificate == "" || config.key == "" || config.serverName == "" || validateListen(config.listen) != nil {
		fmt.Fprintln(errOut, "serve requires a valid listen address, database URL file, role map, client CA, TLS certificate/key, and server name")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, config, errOut); err != nil {
		fmt.Fprintln(errOut, "watchhouse-control:", err)
		return 1
	}
	return 0
}

func validateListen(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || net.ParseIP(host) == nil || port == "" || port == "0" {
		return fmt.Errorf("listen must use a literal IP and nonzero port")
	}
	return nil
}

func serve(ctx context.Context, config options, errOut *os.File) error {
	roles, err := authz.Load(config.rolesFile)
	if err != nil {
		return fmt.Errorf("load human role map: %w", err)
	}
	databaseURL, err := secretfile.Read(config.databaseURLFile, 8192)
	if err != nil {
		return fmt.Errorf("read database URL file")
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("database URL is invalid")
	}
	poolConfig.MaxConns = 20
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = map[string]string{}
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "watchhouse-control"
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "10s"
	poolConfig.ConnConfig.RuntimeParams["lock_timeout"] = "5s"
	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "15s"
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return fmt.Errorf("create database pool")
	}
	defer pool.Close()
	if err := pool.Ping(connectCtx); err != nil {
		return fmt.Errorf("database connection failed")
	}
	if err := controlstore.Migrate(connectCtx, pool); err != nil {
		return fmt.Errorf("database migration failed")
	}
	store, err := controlstore.New(pool)
	if err != nil {
		return fmt.Errorf("initialize event store")
	}
	processor, err := controlstore.NewProcessor(store, detection.DefaultConfig(), 50_000)
	if err != nil {
		return fmt.Errorf("initialize event processor")
	}
	tlsConfiguration, err := transport.LoadServerTLS(config.ca, config.certificate, config.key, config.serverName)
	if err != nil {
		return fmt.Errorf("load control TLS: %w", err)
	}
	listener, err := net.Listen("tcp", config.listen)
	if err != nil {
		return fmt.Errorf("listen on configured address: %w", err)
	}
	defer listener.Close()
	server := &http.Server{
		Handler: transport.Handler{Store: processor, Listeners: store, Probes: store, Queries: store, Findings: store, Auditor: store, Roles: roles}, TLSConfig: tlsConfiguration,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 * 1024, ErrorLog: log.New(errOut, "watchhouse-control http: ", log.LstdFlags),
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(errOut, "watchhouse-control listening on %s with mutual TLS\n", listener.Addr())
	err = server.ServeTLS(listener, "", "")
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve mutual TLS endpoint: %w", err)
	}
	<-shutdownDone
	return nil
}
