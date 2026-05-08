package env

import (
	"fmt"
	"os"
	"strings"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

var (
	// Production is a sentinel indicating a production environment.
	Production = &Environment{Name: "production"}
	// Staging is a sentinel indicating a staging environment.
	Staging = &Environment{Name: "staging"}
	// Dev is a sentinel indicating a dev environment.
	Dev = &Environment{Name: "dev"}
	// Local is a sentinel indicating a local environment.
	Local = &Environment{Name: "local"}
	// Test is a sentinal indicating a test environment (such as CI or unit test).
	Test = &Environment{Name: "test"}
)

// Environment describes information about the runtime environment.
type Environment struct {
	Name string
}

var (
	// ProductionDomain is a sentinel indicating the production domain
	ProductionDomain = &Domain{"getcara.ai"}
	// StagingDomain is a sentinel indicating the staging domain
	StagingDomain = &Domain{"staging.getcara.ai"}
	// InternalProductionDomain is a sentinel indicating the production domain for internal services
	InternalProductionDomain = &Domain{"oysterinc.net"}
	// InternalStagingDomain is a sentinel indicating the staging domain for internal services
	InternalStagingDomain = &Domain{"staging.oysterinc.net"}
	// LocalDomain is a sentinel indicating the local domain
	LocalDomain = &Domain{"localhost"}
)

// Domain represents a canonical domain name in which an Oyster service runs.
type Domain struct {
	Domain string
}

func (d Domain) String() string {
	return d.Domain
}

var (
	// ServiceApp is a sentinel representing the agency portal frontend service
	ServiceApp = &Service{
		Name: "app",
	}
	// ServiceAdmin is a sentinel representing the internal admin frontend service
	ServiceAdmin = &Service{
		Name:         "admin",
		InternalOnly: true,
	}
	// ServiceAPI is a sentinel representing the API service
	ServiceAPI = &Service{
		Name:         "api",
		InternalPort: 8080,
	}
	// ServiceClients is a sentinel representing the clients service
	ServiceClients = &Service{
		Name: "clients",
	}
	// ServiceStatics is a sentinel representing the statics service
	ServiceStatics = &Service{
		Name:      "statics",
		SubDomain: "s",
	}
	// ServiceWebhooks is a sentinel representing the webhooks service.
	ServiceWebhooks = &Service{
		Name: "webhooks",
	}
	// ServiceDev is a sentinel representing the internal dev service.
	ServiceDev = &Service{
		Name:         "dev",
		InternalOnly: true,
		InternalPort: 8080,
	}
	// AllServices is a list of all Oyster services
	AllServices = []*Service{
		ServiceApp,
		ServiceAdmin,
		ServiceAPI,
		ServiceClients,
		ServiceDev,
		ServiceStatics,
		ServiceWebhooks,
	}
)

// Service describes an Oyster service.
type Service struct {
	Name       string
	LocalPort  int
	SubDomain  string
	HostSuffix string
	URLEnv     string

	InternalOnly bool
	InternalPort int
}

// GetEnvironment returns one of the sentinel Environment values
// based on the environment this service is running in.
func GetEnvironment() *Environment {
	env := os.Getenv("ENVIRONMENT")
	switch env {
	case Production.Name:
		return Production
	case Staging.Name:
		return Staging
	case Dev.Name:
		return Dev
	}

	if getIsTest() {
		return Test
	}

	if env == "" {
		return Local
	}

	panic(errors.New("unknown environment: %q", env))
}

// GetDomain returns the domain in which this service is running
func GetDomain() *Domain {
	switch GetEnvironment() {
	case Production:
		return ProductionDomain
	case Staging:
		return StagingDomain
	case Dev:
		return &Domain{GetUserDevDomain(CurrentUserName(), CurrentWorktree())}
	default:
		return LocalDomain
	}
}

// GetInternalDomain returns the domain for internal services
func GetInternalDomain() *Domain {
	switch GetEnvironment() {
	case Production:
		return InternalProductionDomain
	case Staging:
		return InternalStagingDomain
	case Dev:
		return &Domain{GetUserDevDomain(CurrentUserName(), CurrentWorktree())}
	default:
		return LocalDomain
	}
}

// GetSubDomain returns the subdomain on which this service runs.
func (s *Service) GetSubDomain() string {
	if s.SubDomain != "" {
		return s.SubDomain
	}
	return s.Name
}

// GetURLEnv returns the environment variable name for this service
// that would contain its URL override.
func (s *Service) GetURLEnv() string {
	if s.URLEnv != "" {
		return s.URLEnv
	}
	return fmt.Sprintf("%s_HOST_URL", strings.ToUpper(s.Name))
}

// ExternalURL returns the external URL (scheme + host + path prefix) of the service
// so that it can be accessed outside of Oyster infrastructure.
func (s *Service) ExternalURL() string {
	if hostURL := os.Getenv(s.GetURLEnv()); hostURL != "" {
		return hostURL
	}

	if GetEnvironment() == Test {
		return fmt.Sprintf("http://%s:%d%s", LocalDomain, s.LocalPort, s.HostSuffix)
	}

	if s.InternalOnly {
		return fmt.Sprintf("https://%s.%s%s", s.GetSubDomain(), GetInternalDomain(), s.HostSuffix)
	}

	if GetEnvironment() == Dev {
		// Use `s.Name` instead of `s.GetSubDomain()` in dev because of how
		// the ingress service routes directly based on k8s service name instead
		// of reverse-mapping from subdomain tot service name.
		return fmt.Sprintf("https://%s.%s%s", s.Name, GetDomain(), s.HostSuffix)
	}

	// Use route53 routing if this is a public service.
	return fmt.Sprintf("https://%s.%s%s", s.GetSubDomain(), GetDomain(), s.HostSuffix)
}

func (s *Service) InternalURL() string {
	if GetEnvironment() == Test {
		if s.InternalPort == 0 {
			return ""
		}
		return fmt.Sprintf("http://%s:%d%s", s.Name, s.InternalPort, s.HostSuffix)
	}

	if GetEnvironment() == Dev {
		return fmt.Sprintf("http://%s.%s.svc%s", s.Name, GetDevNamespace(), s.HostSuffix)
	}

	// Use coredns routing if this is an internal-only service.
	return fmt.Sprintf("http://%s.%s.svc%s", s.Name, GetEnvironment().Name, s.HostSuffix)
}

// ServiceName returns the name of the currently-running service.
func CurrentServiceName() string {
	return os.Getenv("OYSTER_SERVICE")
}

// CurrentUserName returns the user of the dev environment in which this service is running.
func CurrentUserName() string {
	return os.Getenv("OYSTER_USER")
}

// CurrentWorktree returns the worktree of the dev environment in which this service is running.
func CurrentWorktree() string {
	return os.Getenv("OYSTER_WORKTREE")
}

func GetUserDevDomain(user, worktree string) string {
	if worktree == "" {
		return fmt.Sprintf("%s.dev.oysterinc.net", user)
	}
	return fmt.Sprintf("%s.%s.dev.oysterinc.net", worktree, user)
}

// GetDevNamespace returns the namespace for the dev environment in which this service is running.
func GetDevNamespace() string {
	if CurrentWorktree() == "" {
		return fmt.Sprintf("dev-%s", CurrentUserName())
	}
	return fmt.Sprintf("dev-%s-%s", CurrentUserName(), CurrentWorktree())
}

// IngressAPIKeyPrefix returns a stable environment-derived prefix used in API keys
// so that the ingress can route requests based on the `X-API-KEY` header.
// Examples: "production", "staging", "dev-<user>", "local".
func IngressAPIKeyPrefix() string {
	switch GetEnvironment() {
	case Production:
		return "production"
	case Staging:
		return "staging"
	case Dev:
		user := strings.TrimSpace(CurrentUserName())
		if user == "" {
			user = "dev"
		}
		return fmt.Sprintf("dev-%s", user)
	default:
		return "local"
	}
}
