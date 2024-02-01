package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
	"github.com/anchore/grype/grype/db"
	"github.com/anchore/grype/grype/grypeerr"
	"github.com/anchore/grype/grype/matcher"
	"github.com/anchore/grype/grype/matcher/dotnet"
	"github.com/anchore/grype/grype/matcher/golang"
	"github.com/anchore/grype/grype/matcher/java"
	"github.com/anchore/grype/grype/matcher/javascript"
	"github.com/anchore/grype/grype/matcher/python"
	"github.com/anchore/grype/grype/matcher/ruby"
	"github.com/anchore/grype/grype/matcher/stock"
	grype_pkg "github.com/anchore/grype/grype/pkg"
	"github.com/anchore/grype/grype/presenter/models"
	"github.com/anchore/grype/grype/store"
	"github.com/anchore/syft/syft/format"
	log "github.com/sirupsen/logrus"

	"github.com/openclarity/grype-server/grype-server/pkg/rest"
)

const (
	// From https://github.com/anchore/grype/blob/v0.50.1/internal/config/datasources.go#L10
	defaultMavenBaseURL = "https://search.maven.org/solrsearch/select"
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
	exclusionProvider   *db.MatchExclusionProvider
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
	syftSbom, _, _, err := format.Decode(sbomReader)
	if err != nil {
		return nil, fmt.Errorf("unable to decode sbom: %v", err)
	}

	if syftSbom.Artifacts.Packages == nil {
		return nil, fmt.Errorf("packagecatalog is empty")
	}

	packages := grype_pkg.FromCollection(syftSbom.Artifacts.Packages, grype_pkg.SynthesisConfig{
		GenerateMissingCPEs: true,
	})
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
	vulnerabilityMatcher := createVulnerabilityMatcher(store.Store{
		Provider:          s.vulProvider,
		MetadataProvider:  s.vulMetadataProvider,
		ExclusionProvider: s.exclusionProvider,
	})

	allMatches, ignoredMatches, err := vulnerabilityMatcher.FindMatches(packages, packagesContext)
	// We can ignore ErrAboveSeverityThreshold since we are not setting the FailSeverity on the matcher.
	if err != nil && !errors.Is(err, grypeerr.ErrAboveSeverityThreshold) {
		return nil, fmt.Errorf("failed to find vulnerabilities: %v", err)
	}

	doc, err := models.NewDocument(clio.Identification{}, packages, packagesContext, *allMatches, ignoredMatches, s.vulMetadataProvider, nil, s.dbCurator.Status())
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %v", err)
	}

	return &doc, nil
}

func createVulnerabilityMatcher(store store.Store) *grype.VulnerabilityMatcher {
	matchers := matcher.NewDefaultMatchers(matcher.Config{
		Java: java.MatcherConfig{
			ExternalSearchConfig: java.ExternalSearchConfig{
				// Disable searching maven external source (this is the default for grype CLI too)
				SearchMavenUpstream: false,
				MavenBaseURL:        defaultMavenBaseURL,
			},
			UseCPEs: true,
		},
		Ruby: ruby.MatcherConfig{
			UseCPEs: true,
		},
		Python: python.MatcherConfig{
			UseCPEs: true,
		},
		Dotnet: dotnet.MatcherConfig{
			UseCPEs: true,
		},
		Javascript: javascript.MatcherConfig{
			UseCPEs: true,
		},
		Golang: golang.MatcherConfig{
			UseCPEs: true,
		},
		Stock: stock.MatcherConfig{
			UseCPEs: true,
		},
	})
	return &grype.VulnerabilityMatcher{
		Store:          store,
		Matchers:       matchers,
		NormalizeByCVE: true,
	}
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
	// Inside the dbCurator.Update() the dbCurator.IsUpdateAvailable() is called, and if it returns an error the update won't stop just log the error.
	// https://github.com/anchore/grype/blob/731abaab723ae8918635d4e20399ca3c00b665f4/grype/db/curator.go#L138-L143
	if _, _, _, err := dbCurator.IsUpdateAvailable(); err != nil {
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
	storeReader, _, err := dbCurator.GetStore()

	if err != nil {
		return fmt.Errorf("failed to get store: %v", err)
	}

	s.vulProvider, err = db.NewVulnerabilityProvider(storeReader)
	if err != nil {
		return fmt.Errorf("failed to get vulnerability provider: %v", err)
	}
	s.vulMetadataProvider = db.NewVulnerabilityMetadataProvider(storeReader)
	s.exclusionProvider = db.NewMatchExclusionProvider(storeReader)
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
