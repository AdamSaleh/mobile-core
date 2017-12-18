package data

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aerogear/mobile-core/pkg/mobile"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v1 "k8s.io/client-go/pkg/api/v1"
)

const apiKeyMapName = "mcp-mobile-keys"
const apiKeyMapDisplayName = "API Keys"

// MobileAppValidator defines what a validator should do
type MobileAppValidator interface {
	PreCreate(a *mobile.App) error
	PreUpdate(old *mobile.App, new *mobile.App) error
}

// MobileAppRepo interacts with the data store that backs the mobile objects
type MobileAppRepo struct {
	client       corev1.ConfigMapInterface
	apiKeyClient corev1.SecretInterface
	validator    MobileAppValidator
}

// NewMobileAppRepo instansiates a new MobileAppRepo
func NewMobileAppRepo(c corev1.ConfigMapInterface, apiKeyClient corev1.SecretInterface, v MobileAppValidator) *MobileAppRepo {
	rep := &MobileAppRepo{
		client:       c,
		apiKeyClient: apiKeyClient,
		validator:    v,
	}
	if rep.validator == nil {
		rep.validator = &DefaultMobileAppValidator{}
	}
	return rep
}

// ReadByName attempts to read a mobile app by its unique name
func (mar *MobileAppRepo) ReadByName(name string) (*mobile.App, error) {
	_, cm, err := mar.readMobileAppAndConfigMap(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve mobile app ")
	}
	return convertConfigMapToMobileApp(cm), nil
}

// Create creates a mobile app object. Fails on duplicates
func (mar *MobileAppRepo) Create(app *mobile.App) error {
	if err := mar.validator.PreCreate(app); err != nil {
		return errors.Wrap(err, "validation failed during create")
	}
	app.ID = app.Name + "-" + fmt.Sprintf("%v", time.Now().Unix())
	app.MetaData["created"] = time.Now().Format("2006-01-02 15:04:05")
	cm := convertMobileAppToConfigMap(app)
	if _, err := mar.client.Create(cm); err != nil {
		return errors.Wrap(err, "failed to create underlying configmap for mobile app")
	}
	return nil
}

//DeleteByName will delte the underlying configmap
func (mar *MobileAppRepo) DeleteByName(name string) error {
	return mar.client.Delete(name, &meta_v1.DeleteOptions{})
}

//List will list the configmaps and convert them to mobileapps
func (mar *MobileAppRepo) List() ([]*mobile.App, error) {
	list, err := mar.client.List(meta_v1.ListOptions{LabelSelector: "group=mobileapp"})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list mobileapp configmaps")
	}

	var apps = []*mobile.App{}
	for _, a := range list.Items {
		apps = append(apps, convertConfigMapToMobileApp(&a))
	}
	return apps, nil
}

// Update will update the underlying configmap with the new details
func (mar *MobileAppRepo) Update(app *mobile.App) (*mobile.App, error) {
	old, cm, err := mar.readMobileAppAndConfigMap(app.Name)
	if err != nil {
		return nil, err
	}
	if err := mar.validator.PreUpdate(old, app); err != nil {
		return nil, errors.Wrap(err, "validation failed before update")
	}
	cm.Data["name"] = app.Name
	cm.Data["clientType"] = app.ClientType
	var cmap *v1.ConfigMap
	cmap, err = mar.client.Update(cm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update mobile app configmap")
	}
	return convertConfigMapToMobileApp(cmap), nil
}

// CreateAPIKeyMap Create the API Key Map
func (mar *MobileAppRepo) CreateAPIKeyMap() error {
	_, err := mar.apiKeyClient.Get(apiKeyMapName, meta_v1.GetOptions{})
	if err != nil {
		_, err := mar.apiKeyClient.Create(&v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: apiKeyMapName,
			},
			Data: map[string][]byte{
				"name":        []byte(apiKeyMapName),
				"type":        []byte(apiKeyMapName),
				"displayName": []byte(apiKeyMapDisplayName),
				"apiKeys":     emptyAPIKeyMap(),
			},
		})
		return err
	}
	return nil
}

// AddAPIKeyToMap Add an apps API key to the API Key Map
func (mar *MobileAppRepo) AddAPIKeyToMap(app *mobile.App) error {
	sec, err := mar.apiKeyClient.Get(apiKeyMapName, meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Adding API Key to map, could not retrieve map")
	}
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	if sec.Data["apiKeys"] == nil {
		sec.Data["apiKeys"] = emptyAPIKeyMap()
	}

	apiKeys := map[string]string{}
	err = json.Unmarshal(sec.Data["apiKeys"], &apiKeys)
	if err != nil {
		return errors.Wrap(err, "Adding API Key to map, could not unmarshal key map")
	}

	apiKeys[app.ID] = app.APIKey
	apiKeysJSON, err := json.Marshal(apiKeys)
	if err != nil {
		return errors.Wrap(err, "Adding API Key to map, could not marshal key map")
	}

	sec.Data["apiKeys"] = apiKeysJSON
	if _, err := mar.apiKeyClient.Update(sec); err != nil {
		return errors.Wrap(err, "Adding API Key to map, could not update map")
	}
	return nil
}

// RemoveAPIKeyFromMap Remove an apps API key from the API Key Map
func (mar *MobileAppRepo) RemoveAPIKeyFromMap(appID string) error {
	sec, err := mar.apiKeyClient.Get(apiKeyMapName, meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Removing API Key from map, could not retrieve map")
	}
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	if sec.Data["apiKeys"] == nil {
		sec.Data["apiKeys"] = emptyAPIKeyMap()
	}

	apiKeys := map[string]string{}
	err = json.Unmarshal(sec.Data["apiKeys"], &apiKeys)
	if err != nil {
		return errors.Wrap(err, "Removing API Key from map, could not unmarshal key map")
	}

	delete(apiKeys, appID)
	apiKeysJSON, err := json.Marshal(apiKeys)
	if err != nil {
		return errors.Wrap(err, "Removing API Key from map, could not marshal key map")
	}

	sec.Data["apiKeys"] = apiKeysJSON
	if _, err := mar.apiKeyClient.Update(sec); err != nil {
		return errors.Wrap(err, "Removing API Key from map, could not update map")
	}
	return nil
}

func convertConfigMapToMobileApp(m *v1.ConfigMap) *mobile.App {
	return &mobile.App{
		ID:          m.Name,
		Name:        m.Data["name"],
		DisplayName: m.Data["displayName"],
		ClientType:  m.Data["clientType"],
		APIKey:      m.Data["apiKey"],
		Labels:      m.Labels,
		Description: m.Data["description"],
		MetaData: map[string]string{
			"icon":    m.Annotations["icon"],
			"created": m.Annotations["created"],
		},
	}
}

func convertMobileAppToConfigMap(app *mobile.App) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: app.ID,
			Labels: map[string]string{
				"group": "mobileapp",
				"name":  app.Name,
			},
			Annotations: map[string]string{
				"icon":    app.MetaData["icon"],
				"created": app.MetaData["created"],
			},
		},
		Data: map[string]string{
			"name":        app.Name,
			"displayName": app.DisplayName,
			"clientType":  app.ClientType,
			"apiKey":      app.APIKey,
			"description": app.Description,
		},
	}
}

func (mar *MobileAppRepo) readMobileAppAndConfigMap(name string) (*mobile.App, *v1.ConfigMap, error) {
	cm, err := mar.readUnderlyingConfigMap(&mobile.App{Name: name})
	if err != nil {
		return nil, nil, err
	}
	app := convertConfigMapToMobileApp(cm)
	return app, cm, err
}

func (mar *MobileAppRepo) readUnderlyingConfigMap(a *mobile.App) (*v1.ConfigMap, error) {
	cm, err := mar.client.Get(a.Name, meta_v1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read underlying configmap for app "+a.Name)
	}
	return cm, nil
}

//NewMobileAppRepoBuilder creates a new instance of a MobileAppRepoBuilder
func NewMobileAppRepoBuilder(clientBuilder mobile.K8ClientBuilder, namespace, saToken string) mobile.AppRepoBuilder {
	return &MobileAppRepoBuilder{
		clientBuilder: clientBuilder,
		namespace:     namespace,
		saToken:       saToken,
	}
}

// MobileAppRepoBuilder builds a MobileAppRepo
type MobileAppRepoBuilder struct {
	clientBuilder mobile.K8ClientBuilder
	token         string
	namespace     string
	saToken       string
}

func (marb *MobileAppRepoBuilder) WithToken(t string) mobile.AppRepoBuilder {
	// ensure we get a new instance to avoid reuse of tokens
	return &MobileAppRepoBuilder{
		clientBuilder: marb.clientBuilder,
		token:         t,
		namespace:     marb.namespace,
	}
}

//UseDefaultSAToken delegates off to the service account token setup with the MCP. This should only be used for APIs where no real token is provided and should always be protected
func (marb *MobileAppRepoBuilder) UseDefaultSAToken() mobile.AppRepoBuilder {
	return &MobileAppRepoBuilder{
		clientBuilder: marb.clientBuilder,
		token:         marb.saToken,
		namespace:     marb.namespace,
	}
}

// Build builds the final repo
func (marb *MobileAppRepoBuilder) Build() (mobile.AppCruder, error) {
	k8client, err := marb.clientBuilder.WithToken(marb.token).BuildClient()
	if err != nil {
		return nil, errors.Wrap(err, "MobileAppRepoBuilder failed to build a configmap client")
	}
	return NewMobileAppRepo(k8client.CoreV1().ConfigMaps(marb.namespace), k8client.CoreV1().Secrets(marb.namespace), DefaultMobileAppValidator{}), nil
}

func emptyAPIKeyMap() []byte {
	return []byte("{}")
}
