package plaid

import (
	"log"

	"github.com/plaid/plaid-go/plaid"
)

func NewPlaidClient(clientID, secret, env string) *plaid.APIClient {
	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	configuration.AddDefaultHeader("PLAID-SECRET", secret)

	switch env {
	case "sandbox":
		configuration.UseEnvironment(plaid.Sandbox)
	case "development":
		configuration.UseEnvironment(plaid.Development)
	case "production":
		configuration.UseEnvironment(plaid.Production)
	default:
		log.Fatalf("Invalid Plaid environment: %s", env)
	}

	return plaid.NewAPIClient(configuration)
}
