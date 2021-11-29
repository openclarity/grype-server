package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/anchore/grype/grype"
	grype_pkg "github.com/anchore/grype/grype/pkg"
	"github.com/anchore/grype/grype/presenter/models"
	"github.com/anchore/grype/grype/vulnerability"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"

	"github.com/Portshift/grype-server/grype-server/pkg/rest"
	"github.com/anchore/grype/grype/db"
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
	vulProvider         *vulnerability.StoreAdapter
	vulMetadataProvider *vulnerability.MetadataStoreAdapter
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
		DBRootDir:          s.DbRootDir,
		ListingURL:         s.DbUpdateURL,
		ValidateByHashOnGet: false,
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

func (s *Scanner) Scan(sbom64 string) (string, error) {
	sbom, err := base64.StdEncoding.DecodeString(sbom64)
	if err != nil {
		return "", fmt.Errorf("failed to decode sbom: %v", err)
	}
	log.Tracef("SBOM to scan: %s", sbom)
	doc, err := s.ScanSbomJson(string(sbom))
	if err != nil {
		return "", fmt.Errorf("failed to scan SBOM: %v", err)
	}

	docB, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed marshall SBOM: %v", err)
	}
	log.Tracef("Scan result: %s", docB)
	doc64 := base64.StdEncoding.EncodeToString(docB)

	return doc64, nil
}

func (s *Scanner) ScanSbomJson(sbom string) (*models.Document, error) {
	if s.vulProvider == nil {
		return nil, fmt.Errorf("vulnerability provider wasn't set")
	}

	sbomReader := strings.NewReader(sbom)
	catalog, srcMetadata, distro, _, formatOption, err := syft.Decode(sbomReader)
	if err != nil {
		return nil, fmt.Errorf("unable to decode sbom: %v", err)
	}
	if formatOption == format.UnknownFormatOption {
		return nil, fmt.Errorf("unknown SBOM format option: %v", formatOption)
	}

	packages := grype_pkg.FromCatalog(catalog)
	packagesContext := grype_pkg.Context{
		Source: srcMetadata,
		Distro: distro,
	}

	allMatches := grype.FindVulnerabilitiesForPackage(s.vulProvider, packagesContext.Distro, packages...)

	doc, err := models.NewDocument(packages, packagesContext, allMatches, nil, s.vulMetadataProvider, nil, s.dbCurator.Status())
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %v", err)
	}
	return &doc, nil
}

func (s *Scanner) startUpdateChecker(ctx context.Context) {
	const checkForUpdatesIntervalSec = 10*60 // check every 10 minutes

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
	dbCurator := db.NewCurator(*cfg)
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
		return fmt.Errorf("loaded DB is a has a failed status: %v", status.Err)
	}
	store, err := dbCurator.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get store: %v", err)
	}

	s.vulProvider = vulnerability.NewProviderFromStore(store)
	s.vulMetadataProvider = vulnerability.NewMetadataStoreProvider(store)
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
