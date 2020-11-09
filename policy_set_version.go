package tfe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	slug "github.com/hashicorp/go-slug"
)

// Compile-time proof of interface implementation.
var _ PolicySetVersions = (*policySetVersions)(nil)

// PolicySetVersions describes all the policy set version related
// methods that the Terraform Enterprise API supports.
//
// TFE API docs:
// https://www.terraform.io/docs/enterprise/api/policy-sets.html
type PolicySetVersions interface {

	// Create is used to create a new policy set version. The created
	// policy set version will be usable once data is uploaded to it.
	Create(ctx context.Context, policySetID string, options PolicySetVersionCreateOptions) (*PolicySetVersion, error)

	// Read a policy set version by its ID.
	Read(ctx context.Context, psvID string) (*PolicySetVersion, error)

	// Upload package with Sentinel policies and modules. It requires
	// the upload URL from a policy set version and the full path to the
	// policy set files on disk.
	Upload(ctx context.Context, url string, path string) error
}

// policySetVersions implements PolicySetVersions.
type policySetVersions struct {
	client *Client
}

// PolicySetVersionStatus represents a policy set version status.
type PolicySetVersionStatus string

//List all available policy set version statuses.
const (
	PolicySetVersionErrored  PolicySetVersionStatus = "errored"
	PolicySetVersionPending  PolicySetVersionStatus = "pending"
	PolicySetVersionReady    PolicySetVersionStatus = "ready"
)

// PolicySetVersionSource represents a source of a policy set version.
type PolicySetVersionSource string

// List all available policy set version sources.
const (
	PolicySetVersionSourceAPI       PolicySetVersionSource = "tfe-api"
	PolicySetVersionSourceBitbucket PolicySetVersionSource = "bitbucket"
	PolicySetVersionSourceGithub    PolicySetVersionSource = "github"
	PolicySetVersionSourceGitlab    PolicySetVersionSource = "gitlab"
	PolicySetVersionSourceTerraform PolicySetVersionSource = "terraform"
)

type PolicySetVersion struct {
	Data             *PolicySetVersionData  `json:"data"`
}

type PolicySetVersionData struct {
	Type             string                 `json:"type"`
	ID               string                 `json:"id"`
	Attributes       *PSVAttributes         `json:"attributes"`
	Relationships    *PSVRelationships      `json:"relationships"`
	Links            *PSVLinks              `json:"links"`
}

type PSVAttributes struct {
	Source           PolicySetVersionSource `json:"source"`
	Status           PolicySetVersionStatus `json:"status"`
	StatusTimestamps *PSVStatusTimestamps   `json:"status-timestamps"`
	Error            string                 `json:"error"`
	CreatedAt        time.Time              `json:"created-at"`
	UpdatedAt        time.Time              `json:"updated-at"`
}

type PSVRelationships struct {
	PolicySet *PolicySetRelationship `json:"policy-set"`
}

type PolicySetRelationship struct {
	Data *PolicySetData     `json:"data"`
}

type PolicySetData struct {
	Type string  `json:"type"`
	ID   string  `json:"id"`
}

type PSVLinks struct {
	Self   string `json:"self"`
	Upload string `json:"upload"`
}

// PSVStatusTimestamps holds the timestamps for individual policy set version
// statuses.
type PSVStatusTimestamps struct {
	ReadyAt    time.Time `json:"ready-at"`
	PendingAt  time.Time `json:"pending-at"`
	ErroredAt  time.Time `json:"errored-at"`
}

// PolicySetVersionCreateOptions represents the options for creating a
// policy set version.
type PolicySetVersionCreateOptions struct {
	// For internal use only!
	ID string `jsonapi:"primary,policy-set-versions"`
}

// Create is used to create a new policy set version. The created
// policy set version will be usable once data is uploaded to it.
func (s *policySetVersions) Create(ctx context.Context, policySetID string, options PolicySetVersionCreateOptions) (*PolicySetVersion, error) {
	if !validStringID(&policySetID) {
		return nil, errors.New("invalid value for policy set ID")
	}

	// Make sure we don't send a user provided ID.
	options.ID = ""

	u := fmt.Sprintf("policy-sets/%s/versions", url.QueryEscape(policySetID))
	req, err := s.client.newRequest("POST", u, &options)
	if err != nil {
		return nil, err
	}

	psv := &PolicySetVersion{}
	err = s.client.do(ctx, req, psv)
	if err != nil {
		return nil, err
	}

	return psv, nil
}

// Read a policy set version by its ID.
func (s *policySetVersions) Read(ctx context.Context, psvID string) (*PolicySetVersion, error) {
	if !validStringID(&psvID) {
		return nil, errors.New("invalid value for policy set version ID")
	}

	u := fmt.Sprintf("policy-set-versions/%s", url.QueryEscape(psvID))
	req, err := s.client.newRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	psv := &PolicySetVersion{}
	err = s.client.do(ctx, req, psv)
	if err != nil {
		return nil, err
	}

	return psv, nil
}

// Upload package with Sentinel policies and modules. It requires the
// upload URL from a policy set version and the full path to the policy set
// files on disk.
func (s *policySetVersions) Upload(ctx context.Context, url, path string) error {
	file, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !file.Mode().IsDir() {
		return errors.New("path needs to be an existing directory")
	}

	body := bytes.NewBuffer(nil)

	_, err = slug.Pack(path, body, true)
	if err != nil {
		return err
	}

	req, err := s.client.newRequest("PUT", url, body)
	if err != nil {
		return err
	}

	return s.client.do(ctx, req, nil)
}
