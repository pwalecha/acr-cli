// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Azure/go-autorest/autorest"

	acrapi "github.com/Azure/acr-cli/acr"
)

const (
	prefixHTTPS           = "https://"
	registryURL           = ".azurecr.io"
	manifestTagFetchCount = 100
)

type AcrCLIClient struct {
	AutorestClient        acrapi.BaseClient
	manifestTagFetchCount int32
}

// BearerAuth returns the authentication header in case an access token was specified.
func BearerAuth(accessToken string) string {
	return "Bearer " + accessToken
}

// BasicAuth returns the username and the passwrod encoded in base 64.
func BasicAuth(username string, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// LoginURL returns the FQDN for a registry.
func LoginURL(registryName string) string {
	// TODO: if the registry is in another cloud (i.e. dogfood) a full FQDN for the registry should be specified.
	if strings.Contains(registryName, ".") {
		return registryName
	}
	return registryName + registryURL
}

// LoginURLWithPrefix return the hostname of a registry.
func LoginURLWithPrefix(loginURL string) string {
	urlWithPrefix := loginURL
	if !strings.HasPrefix(loginURL, prefixHTTPS) {
		urlWithPrefix = prefixHTTPS + loginURL
	}
	return urlWithPrefix
}

func NewAcrCLIClient(loginURL string) AcrCLIClient {
	loginURL = LoginURLWithPrefix(loginURL)
	return AcrCLIClient{
		acrapi.NewWithoutDefaults(loginURL),
		100,
	}
}

func NewAcrCLIClientWithBasicAuth(loginURL string, username string, password string) AcrCLIClient {
	newAcrCLIClient := NewAcrCLIClient(loginURL)
	newAcrCLIClient.AutorestClient.Authorizer = autorest.NewBasicAuthorizer(username, password)
	return newAcrCLIClient
}

// AcrListTags list the tags of a repository with their attributes.
func (c *AcrCLIClient) GetAcrTags(ctx context.Context, repoName string, orderBy string, last string) (*acrapi.RepositoryTagsType, error) {
	tags, err := c.AutorestClient.GetAcrTags(ctx, repoName, last, &c.manifestTagFetchCount, orderBy, "")
	if err != nil {
		return nil, err
	}
	return &tags, nil
}

// DeleteTag deletes the tag by reference.
func (c *AcrCLIClient) DeleteAcrTag(ctx context.Context, repoName string, reference string) error {
	_, err := c.AutorestClient.DeleteAcrTag(ctx, repoName, reference)
	if err != nil {
		return err
	}
	return nil
}

// ListManifestsAttributes list all the manifest in a repository with their attributes.
func (c *AcrCLIClient) GetAcrManifests(ctx context.Context, repoName string, orderBy string, last string) (*acrapi.Manifests, error) {
	manifests, err := c.AutorestClient.GetAcrManifests(ctx, repoName, last, &c.manifestTagFetchCount, orderBy)
	if err != nil {
		return nil, err
	}
	return &manifests, nil
}

// DeleteManifestByDigest deletes a manifest using the digest as a reference.
func (c *AcrCLIClient) DeleteManifest(ctx context.Context, repoName string, reference string) error {
	_, err := c.AutorestClient.DeleteManifest(ctx, repoName, reference)
	if err != nil {
		return err
	}
	return nil
}

func (c *AcrCLIClient) GetAcrManifestMetadata(ctx context.Context, repoName string, reference string) (string, error) {
	metadataResponse, err := c.AutorestClient.GetAcrManifestMetadata(ctx, repoName, reference, "acrarchiveinfo")
	if err != nil {
		return "", err
	}
	metadataBytes, err := json.Marshal(metadataResponse.Value)
	if err != nil {
		return "", err
	}
	return string(metadataBytes), nil
}

func (c *AcrCLIClient) UpdateAcrManifestMetadata(ctx context.Context, repoName string, reference string, metadataValue interface{}) error {
	_, err := c.AutorestClient.UpdateAcrManifestMetadata(ctx, repoName, reference, "acrarchiveinfo", &metadataValue)
	if err != nil {
		return err
	}
	return nil
}

func (c *AcrCLIClient) GetManifest(ctx context.Context, repoName string, reference string) ([]byte, error) {
	var result acrapi.SetObject
	req, err := c.AutorestClient.GetManifestPreparer(ctx, repoName, reference, "application/vnd.docker.distribution.manifest.v2+json")
	if err != nil {
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "GetManifest", nil, "Failure preparing request")
		return nil, err
	}

	resp, err := c.AutorestClient.GetManifestSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "GetManifest", resp, "Failure sending request")
		return nil, err
	}

	var manifestBytes []byte
	if resp.Body != nil {
		manifestBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	}

	resp.Body = ioutil.NopCloser(bytes.NewBuffer(manifestBytes))

	_, err = c.AutorestClient.GetManifestResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "GetManifest", resp, "Failure responding to request")
		return nil, err
	}

	return manifestBytes, nil
}

func (c *AcrCLIClient) AcrCrossReferenceLayer(ctx context.Context, repoName string, reference string, repoFrom string) error {
	_, err := c.AutorestClient.StartBlobUpload(ctx, repoName, "", repoFrom, reference)
	if err != nil {
		return err
	}
	return nil
}

func (c *AcrCLIClient) PutManifest(ctx context.Context, repoName string, reference string, manifest string) error {
	var result autorest.Response
	urlParameters := map[string]interface{}{
		"url": c.AutorestClient.LoginURI,
	}

	pathParameters := map[string]interface{}{
		"name":      autorest.Encode("path", repoName),
		"reference": autorest.Encode("path", reference),
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/vnd.docker.distribution.manifest.v2+json"),
		autorest.AsPut(),
		autorest.WithCustomBaseURL("{url}", urlParameters),
		autorest.WithPathParameters("/v2/{name}/manifests/{reference}", pathParameters),
		autorest.WithString(manifest))

	req, err := preparer.Prepare((&http.Request{}).WithContext(ctx))

	if err != nil {
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "PutManifest", nil, "Failure preparing request")
		return err
	}

	resp, err := c.AutorestClient.PutManifestSender(req)
	if err != nil {
		result.Response = resp
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "PutManifest", resp, "Failure sending request")
		return err
	}

	result, err = c.AutorestClient.PutManifestResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "acr.BaseClient", "PutManifest", resp, "Failure responding to request")
		return err
	}
	return nil
}

func (c *AcrCLIClient) UpdateAcrTagMetadata(ctx context.Context, repoName string, reference string, metadataValue interface{}) error {
	_, err := c.AutorestClient.UpdateAcrTagMetadata(ctx, repoName, reference, "acrarchiveinfo", &metadataValue)
	if err != nil {
		return err
	}
	return nil
}

type AcrCLIClientInterface interface {
	GetAcrTags(ctx context.Context, repoName string, orderBy string, last string) (*acrapi.RepositoryTagsType, error)
	DeleteAcrTag(ctx context.Context, repoName string, reference string) error
	GetAcrManifests(ctx context.Context, repoName string, orderBy string, last string) (*acrapi.Manifests, error)
	DeleteManifest(ctx context.Context, repoName string, reference string)
	GetAcrManifestMetadata(ctx context.Context, repoName string, reference string)
	UpdateAcrManifestMetadata(ctx context.Context, repoName string, reference string, metadataValue string)
	GetManifest(ctx context.Context, repoName string, reference string)
	AcrCrossReferenceLayer(ctx context.Context, repoName string, reference string, repoFrom string)
	PutManifest(ctx context.Context, repoName string, reference string, manifest acrapi.Manifest)
}
