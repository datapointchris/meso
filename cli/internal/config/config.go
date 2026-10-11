// Package config says where the meso CLI is pointed and what it hands
// goclilogin. Where it is pointed is one file, $XDG_CONFIG_HOME/meso/config.toml,
// read through goclilogin's config-file contract: its api, issuer and client_id
// keys, with --api the only thing that outranks them.
package config

import (
	"context"
	"slices"

	"github.com/datapointchris/goclilogin"
)

// product names the config file's directory and the state directory.
const product = "meso"

// keyringService namespaces meso's entries in the OS keychain. It is a deployed
// identifier rather than a path-derived one, so it keeps its spelling
// independently of the module or binary name.
const keyringService = "meso-cli"

// loginScopes are goclilogin's defaults plus groups, which puts the caller's
// groups in the access token so the API can admit the admin group to `meso admin`.
var loginScopes = append(slices.Clone(goclilogin.DefaultScopes), "groups")

// Load reads the deployment every command talks to. api is the --api flag's
// value, and outranks the file's when it is not empty.
func Load(ctx context.Context, api string) (goclilogin.Deployment, error) {
	return goclilogin.LoadDeployment(ctx, product, api)
}

// ForLogin is the deployment `meso auth login` logs in to: the file's values,
// with whatever it lacks filled in from api, the API's metadata and clientID.
func ForLogin(ctx context.Context, api, clientID string) (goclilogin.Deployment, error) {
	return goclilogin.DeploymentForLogin(ctx, product, api, clientID)
}

// DefaultClientID is the client id login derives when neither the file nor
// --client-id names one: meso-cli-<short hostname>.
func DefaultClientID() string {
	return goclilogin.ClientID(product)
}

// Login is the goclilogin view of a deployment: which provider to authenticate
// against, as which client, and where its state lives. StateDir holds the
// refresh lock and the fallback token file, and naming the tool's own directory
// keeps both beside its other state rather than under the keyring service name,
// which is where goclilogin would put them by default.
func Login(d goclilogin.Deployment) goclilogin.Config {
	return goclilogin.Config{
		Issuer:         d.Issuer.Value,
		ClientID:       d.ClientID.Value,
		KeyringService: keyringService,
		StateDir:       goclilogin.StateDir(product),
		Scopes:         loginScopes,
	}
}
