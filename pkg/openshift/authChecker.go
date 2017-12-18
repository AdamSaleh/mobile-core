package openshift

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"bytes"

	"github.com/Sirupsen/logrus"
	"github.com/aerogear/mobile-core/pkg/mobile"
	"github.com/pkg/errors"
)

//AuthCheckerBuilder for building AuthCheckers
type AuthCheckerBuilder struct {
	Host          string
	Token         string
	SkipCertCheck bool
	UserRepo      mobile.UserRepo
}

// AuthChecker checks authorizations against resource in namespaces
type AuthChecker struct {
	Host          string
	Token         string
	SkipCertCheck bool
	UserRepo      mobile.UserRepo
}

// Build an AuthChecker and return it
func (acb *AuthCheckerBuilder) Build() mobile.AuthChecker {
	return &AuthChecker{
		Host:          acb.Host,
		Token:         acb.Token,
		SkipCertCheck: acb.SkipCertCheck,
		UserRepo:      acb.UserRepo,
	}
}

// IgnoreCerts sets the config to ignore future certificate errors
func (acb *AuthCheckerBuilder) IgnoreCerts() mobile.AuthCheckerBuilder {
	return &AuthCheckerBuilder{
		Host:          acb.Host,
		Token:         acb.Token,
		SkipCertCheck: true,
		UserRepo:      acb.UserRepo,
	}
}

// WithToken stores the provided for creating future AuthCheckers
func (acb *AuthCheckerBuilder) WithToken(token string) mobile.AuthCheckerBuilder {
	return &AuthCheckerBuilder{
		Host:          acb.Host,
		SkipCertCheck: acb.SkipCertCheck,
		Token:         token,
		UserRepo:      acb.UserRepo,
	}
}

// WithUserRepo stores the provided userrrepo for creating future AuthCheckers
func (acb *AuthCheckerBuilder) WithUserRepo(repo mobile.UserRepo) mobile.AuthCheckerBuilder {
	return &AuthCheckerBuilder{
		Host:          acb.Host,
		SkipCertCheck: acb.SkipCertCheck,
		Token:         acb.Token,
		UserRepo:      repo,
	}
}

type authCheckJsonPayload struct {
	Verb     string `json:"verb"`
	Resource string `json:"resource"`
}

type authCheckResponse struct {
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

// Check that the resource in the provided namespace can be written to by the current user
func (ac *AuthChecker) Check(resource, namespace string, client mobile.ExternalHTTPRequester) (bool, error) {
	user, err := ac.UserRepo.GetUser()
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to retrieve user details")
	}
	u, err := url.Parse(ac.Host)
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to parse openshift host when attempting to check authorization")
	}
	u.Path = path.Join("/oapi/v1/namespaces/" + namespace + "/localresourceaccessreviews")
	payload := authCheckJsonPayload{
		Verb:     "update",
		Resource: "deploymentconfigs",
	}
	bytePayload, err := json.Marshal(payload)
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to build payload for check authorization")
	}
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(bytePayload))
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to build request to check authorization")
	}
	req.Header.Set("authorization", "bearer "+ac.Token)
	req.Header.Set("Content-Type", "Application/JSON")
	resp, err := client.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to make request to check authorization")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error("failed to close response body. can cause file handle leaks ", err)
		}
	}()
	if resp.StatusCode == http.StatusForbidden {
		// user does not have permission to create the permission check in the namespace
		return false, nil
	} else if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return false, &AuthenticationError{Message: "openshift.ac.Check -> (" + strconv.Itoa(resp.StatusCode) + ") access was denied", StatusCode: resp.StatusCode}
		}

		return false, errors.New("openshift.ac.Check -> unexpected response code from openshift " + strconv.Itoa(resp.StatusCode))
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> failed to read the response body after reading user")
	}
	res := &authCheckResponse{}
	err = json.Unmarshal(data, res)
	if err != nil {
		return false, errors.Wrap(err, "openshift.ac.Check -> error decoding response to auth check")
	}
	for _, u := range res.Users {
		if u == user.User {
			return true, nil
		}
	}

	return user.InAnyGroup(res.Groups), nil
}

// NewAuthCheckerBuilder created and returned with the provided namespace and host
func NewAuthCheckerBuilder(host string) mobile.AuthCheckerBuilder {
	return &AuthCheckerBuilder{
		Host:          host,
		SkipCertCheck: false,
	}
}
