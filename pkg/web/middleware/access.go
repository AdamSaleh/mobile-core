package middleware

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/aerogear/mobile-core/pkg/mobile"
	"github.com/aerogear/mobile-core/pkg/openshift"
	"github.com/aerogear/mobile-core/pkg/web/headers"
	"github.com/pkg/errors"
)

type UserChecker func(host, token string, skipTLS bool) (*mobile.User, error)

// Access handles the cross origin requests
type Access struct {
	host      string
	logger    *logrus.Logger
	userCheck UserChecker
}

func NewAccess(logger *logrus.Logger, host string, userCheck UserChecker) *Access {
	return &Access{
		logger:    logger,
		host:      host,
		userCheck: userCheck,
	}
}

func buildIgnoreList() []*regexp.Regexp {
	cfg := regexp.MustCompile("GET:/config.js")
	sdk := regexp.MustCompile("GET:/sdk/mobileapp/.*/config")
	ping := regexp.MustCompile("GET:/sys/info/ping")
	health := regexp.MustCompile("GET:/sys/info/health")
	metrics := regexp.MustCompile("GET:/metrics")
	download := regexp.MustCompile("GET:/build/.*/download")
	return []*regexp.Regexp{
		download,
		cfg,
		sdk,
		ping,
		health,
		metrics,
	}
}

var ingnoreList = buildIgnoreList()

func shouldIgnore(path string) bool {
	for _, i := range ingnoreList {
		if i.Match([]byte(path)) {
			return true
		}
	}
	return false
}

// Handle sets the required headers
func (c Access) Handle(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	token := headers.DefaultTokenRetriever(req.Header)
	if shouldIgnore(req.Method + ":" + req.URL.Path) {
		next(w, req)
		return
	}
	if token == "" {
		http.Error(w, "no token provided access denied", 401)
		return
	}
	//todo take config to set skipTLS
	if _, err := c.userCheck(c.host, token, true); err != nil {
		if openshift.IsAuthenticationError(err) {
			c.logger.Error(errors.Wrap(err, " access check: checking user is authenticated"))
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		c.logger.Error(fmt.Sprintf("failed to read user to check access %s : %+v", err.Error(), err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, req)
	return

}
