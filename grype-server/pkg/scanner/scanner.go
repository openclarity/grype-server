package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anchore/grype/grype"
	"github.com/anchore/grype/grype/db"
	grype_pkg "github.com/anchore/grype/grype/pkg"
	"github.com/anchore/grype/grype/presenter/models"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format"
	log "github.com/sirupsen/logrus"

	"github.com/Portshift/grype-server/grype-server/pkg/rest"
)

type Config struct {
	RestServerPort int
	DbRootDir      string
	DbUpdateURL    string
}

type Scanner struct {
	restServer  *rest.Server
	dbCurator   *db.Curator
	DbRootDir   string
	DbUpdateURL string

	sync.RWMutex
	vulProvider         *db.VulnerabilityProvider
	vulMetadataProvider *db.VulnerabilityMetadataProvider
}

func Create(conf *Config) (*Scanner, error) {
	var err error
	s := &Scanner{
		DbRootDir:   conf.DbRootDir,
		DbUpdateURL: conf.DbUpdateURL,
	}

	s.restServer, err = rest.CreateRESTServer(conf.RestServerPort, s)
	if err != nil {
		return nil, fmt.Errorf("failed to start rest server: %v", err)
	}

	return s, nil
}

func (s *Scanner) Start(ctx context.Context, errChan chan struct{}) error {
	dbConfig := &db.Config{
		DBRootDir:           s.DbRootDir,
		ListingURL:          s.DbUpdateURL,
		ValidateByHashOnGet: false, // Don't validate the checksum of the DB file after DB update
	}
	if err := s.loadBb(dbConfig); err != nil {
		return fmt.Errorf("failed to load DB: %v", err)
	}
	s.startUpdateChecker(ctx)
	s.restServer.Start(errChan)

	return nil
}

func (s *Scanner) Stop() {
	if s.restServer != nil {
		s.restServer.Stop()
	}
}

func (s *Scanner) Scan(sbom []byte) ([]byte, error) {
	log.Tracef("SBOM to scan: %s", sbom)
	doc, err := s.ScanSbomJson(string(sbom))
	if err != nil {
		return nil, fmt.Errorf("failed to scan SBOM: %v", err)
	}

	docB, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scan result: %v", err)
	}
	log.Tracef("Scan result: %s", docB)

	return docB, nil
}

func (s *Scanner) ScanSbomJson(sbom string) (*models.Document, error) {
	if s.vulProvider == nil {
		return nil, fmt.Errorf("vulnerability provider wasn't set")
	}

	sbomReader := strings.NewReader(sbom)
	syftSbom, formatOption, err := syft.Decode(sbomReader)
	if err != nil {
		return nil, fmt.Errorf("unable to decode sbom: %v", err)
	}
	if formatOption == format.UnknownFormatOption {
		return nil, fmt.Errorf("unknown SBOM format option: %v", formatOption)
	}

	packages := grype_pkg.FromCatalog(syftSbom.Artifacts.PackageCatalog)
	packagesContext := grype_pkg.Context{
		Source: &syftSbom.Source,
		Distro: syftSbom.Artifacts.LinuxDistribution,
	}

	doc, err := s.scanWithRetries(packagesContext, packages)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Scanner) scan(packagesContext grype_pkg.Context, packages []grype_pkg.Package) (*models.Document, error) {
	allMatches := grype.FindVulnerabilitiesForPackage(s.vulProvider, packagesContext.Distro, packages...)

	doc, err := models.NewDocument(packages, packagesContext, allMatches, nil, s.vulMetadataProvider, nil, s.dbCurator.Status())
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %v", err)
	}
	return &doc, nil
}

const (
	numOfScanAttempts = 5
	scanRetryInterval = 5 * time.Second
)

func (s *Scanner) scanWithRetries(packagesContext grype_pkg.Context, packages []grype_pkg.Package) (*models.Document, error) {
	var err error
	var ret *models.Document

	for attempt := 1; attempt <= numOfScanAttempts; attempt++ {
		ret, err = s.scan(packagesContext, packages)
		if err != nil {
			log.Errorf("Failed to scan (attempt %v): %v", attempt, err)
			time.Sleep(scanRetryInterval)
		} else {
			return ret, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan after %v attempts: %v", numOfScanAttempts, err)
	}

	return ret, nil
}

func (s *Scanner) startUpdateChecker(ctx context.Context) {
	const checkForUpdatesIntervalSec = 3 * 60 * 60 // check every 3 hours

	go func() {
		checkServerVersionInterval := checkForUpdatesIntervalSec * time.Second
		for {
			if err := s.updateBb(); err != nil {
				log.Errorf("Failed to update DB: %v", err)
			}
			select {
			case <-ctx.Done():
				log.Debugf("Stopping server version monitor")
				return
			case <-time.After(checkServerVersionInterval):
			}
		}
	}()
}

func (s *Scanner) loadBb(cfg *db.Config) error {
	dbCurator, err := db.NewCurator(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create curator: %v", err)
	}
	// Check IsUpdateAvailable() errors because if it has errors, the update process will be continued during Update().
	// https://github.com/anchore/grype/blob/731abaab723ae8918635d4e20399ca3c00b665f4/grype/db/curator.go#L138-L143
	if _, _, err := dbCurator.IsUpdateAvailable(); err != nil {
		return fmt.Errorf("unable to check for vulnerability database update: %v", err)
	}
	updated, err := dbCurator.Update()
	if err != nil {
		return fmt.Errorf("failed to update DB: %v", err)
	}

	if updated {
		log.Infof("DB was updated")
	} else {
		log.Infof("DB update is not needed")
	}

	status := dbCurator.Status()
	if status.Err != nil {
		return fmt.Errorf("loaded DB has a failed status: %v", status.Err)
	}
	store, err := dbCurator.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get store: %v", err)
	}

	s.vulProvider = db.NewVulnerabilityProvider(store)
	s.vulMetadataProvider = db.NewVulnerabilityMetadataProvider(store)
	s.dbCurator = &dbCurator

	return nil
}

func (s *Scanner) updateBb() error {
	if s.dbCurator == nil {
		return fmt.Errorf("db was not loaded")
	}
	updated, err := s.dbCurator.Update()
	if err != nil {
		return fmt.Errorf("failed to update DB: %v", err)
	}

	if updated {
		log.Infof("DB was updated")
	} else {
		log.Infof("DB update is not needed")
	}

	return nil
}
