package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"

	"github.com/Portshift/go-utils/healthz"
	logutils "github.com/Portshift/go-utils/log"
	"github.com/anchore/grype/grype/vulnerability"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"

	"github.com/openclarity/grype-server/grype-server/pkg/config"
	"github.com/openclarity/grype-server/grype-server/pkg/scanner"
)

const defaultChanSize = 100

func run(c *cli.Context) {
	logutils.InitLogs(c, os.Stdout)
	conf := config.LoadConfig()

	// remove database directory if it exists to avoid using a corrupt database
	dbDir := path.Join(conf.DbRootDir, strconv.Itoa(vulnerability.SchemaVersion))
	if _, err := os.Stat(dbDir); !os.IsNotExist(err) {
		if err = os.RemoveAll(dbDir); err != nil {
			log.Fatalf("Unable to delete existing DB directory: %v", err)
		}
	}

	errChan := make(chan struct{}, defaultChanSize)

	healthServer := healthz.NewHealthServer(conf.HealthCheckAddress)
	healthServer.Start()
	healthServer.SetIsReady(false)

	s, err := scanner.Create(&scanner.Config{
		RestServerPort: conf.RestServerPort,
		DbRootDir:      conf.DbRootDir,
		DbUpdateURL:    conf.DbUpdateURL,
	})
	if err != nil {
		log.Fatalf("Failed to create scanner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx, errChan); err != nil {
		log.Fatalf("Failed to start scanner: %v", err)
	}
	defer s.Stop()

	healthServer.SetIsReady(true)

	// Wait for deactivation
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-errChan:
		log.Errorf("Received an error - shutting down")
	case s := <-sig:
		log.Warningf("Received a termination signal: %v", s)
	}
}

func main() {
	viper.SetDefault(config.RestServerPort, "9991")
	viper.SetDefault(config.HealthCheckAddress, ":8080")
	viper.SetDefault(config.DbRootDir, "/tmp/")
	viper.SetDefault(config.DbUpdateURL, "https://toolbox-data.anchore.io/grype/databases/listing.json")
	viper.SetDefault(config.DbDirName, "3")
	viper.AutomaticEnv()

	app := cli.NewApp()
	app.Usage = ""
	app.Name = "Grype Server"
	app.Version = "1.0.0"

	runCommand := cli.Command{
		Name:   "run",
		Usage:  "Starts Grype Server",
		Action: run,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  logutils.LogLevelFlag,
				Value: logutils.LogLevelDefaultValue,
				Usage: logutils.LogLevelFlagUsage,
			},
		},
	}
	runCommand.UsageText = runCommand.Name

	app.Commands = []cli.Command{
		runCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
