package k8s

import (
	"github.com/aerogear/mobile-core/pkg/mobile"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ClientBuilder struct {
	namespace, token, host string
	inCluster              bool
}

func NewClientBuilder(namespace, host string, incluster bool) mobile.K8ClientBuilder {
	return &ClientBuilder{
		namespace: namespace,
		host:      host,
		inCluster: incluster,
	}
}

func (cb *ClientBuilder) WithToken(token string) mobile.K8ClientBuilder {
	//important to return a new instance
	return &ClientBuilder{
		namespace: cb.namespace,
		token:     token,
		host:      cb.host,
	}
}

func (cb *ClientBuilder) WithNamespace(ns string) mobile.K8ClientBuilder {
	return &ClientBuilder{
		namespace: ns,
		token:     cb.token,
		host:      cb.host,
	}
}
func (cb *ClientBuilder) WithHost(host string) mobile.K8ClientBuilder {
	return &ClientBuilder{
		namespace: cb.namespace,
		token:     cb.token,
		host:      host,
	}
}

func (cb *ClientBuilder) WithHostAndNamespace(host, ns string) mobile.K8ClientBuilder {
	return &ClientBuilder{
		namespace: ns,
		token:     cb.token,
		host:      host,
	}
}

func (cb *ClientBuilder) BuildClient() (kubernetes.Interface, error) {
	if cb.inCluster {
		return ClientForInCluster(cb.token)
	}
	return ClientForOutsideCluster(cb.host, cb.token)
}

func ClientForInCluster(token string) (kubernetes.Interface, error) {
	incluster, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create incluster config for kubernetes client")
	}
	incluster.BearerToken = token
	return kubernetes.NewForConfig(incluster)
}

func ClientForOutsideCluster(host, token string) (kubernetes.Interface, error) {
	conf := clientcmdapi.NewConfig()
	clientConf := clientcmd.NewDefaultClientConfig(*conf, &clientcmd.ConfigOverrides{
		ClusterInfo: clientcmdapi.Cluster{InsecureSkipTLSVerify: true, Server: host},
		AuthInfo:    clientcmdapi.AuthInfo{Token: token},
	})
	rc, err := clientConf.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client config")
	}

	kc, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new kubernetes client ")
	}
	return kc, nil
}
