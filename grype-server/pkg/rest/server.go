package rest

import (
	"fmt"

	"github.com/Portshift/grype-server/api/server/models"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/Portshift/grype-server/api/server/restapi"
	"github.com/Portshift/grype-server/api/server/restapi/operations"
	scanner "github.com/Portshift/grype-server/grype-server/pkg/scanner/interface"
)

type Server struct {
	server *restapi.Server
	scanner.Scanner
}

func CreateRESTServer(port int, scanner scanner.Scanner) (*Server, error) {
	s := &Server{
		Scanner: scanner,
	}

	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to load swagger spec: %v", err)
	}

	api := operations.NewGrypeServerAPI(swaggerSpec)

	api.PostScanSBOMHandler = operations.PostScanSBOMHandlerFunc(func(params operations.PostScanSBOMParams) middleware.Responder {
		log.Infof("Handling Scan request.")
		vulnerabilities, err := s.Scan(params.Body.Sbom)
		if err != nil {
			log.Errorf("Failed to scan SBOM: %v", err)
			return operations.NewPostScanSBOMDefault(500)
		}
		return operations.NewPostScanSBOMOK().WithPayload(&models.Vulnerabilities{
			Vulnerabilities: vulnerabilities,
		})
	})

	server := restapi.NewServer(api)

	server.ConfigureFlags()
	server.ConfigureAPI()
	server.Port = port

	s.server = server

	return s, nil
}

func (s *Server) Start(errChan chan struct{}) {
	log.Infof("Starting REST server")
	go func() {
		if err := s.server.Serve(); err != nil {
			log.Errorf("Failed to serve REST server: %v", err)
			errChan <- struct{}{}
		}
	}()
}

func (s *Server) Stop() {
	log.Infof("Stopping REST server")
	if s.server != nil {
		if err := s.server.Shutdown(); err != nil {
			log.Errorf("Failed to shutdown REST server: %v", err)
		}
	}
}
