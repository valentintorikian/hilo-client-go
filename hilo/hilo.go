package hilo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const BaseUrl = "https://apim.hiloenergie.com/Automation/v1/api"
const SubscriptionKey = "20eeaedcb86945afa3fe792cea89b8bf"

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    string    `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	IdToken      string    `json:"id_token"`
	ExpiryDate   time.Time `json:"-"`
}

func (t Token) Expired() bool {
	return time.Now().After(t.ExpiryDate)
}

type Location struct {
	Id                   int       `json:"id"`
	AddressId            string    `json:"addressId"`
	Name                 string    `json:"name"`
	EnergyCostConfigured bool      `json:"energyCostConfigured"`
	PostalCode           string    `json:"postalCode"`
	CountryCode          string    `json:"countryCode"`
	TemperatureFormat    string    `json:"temperatureFormat"`
	TimeFormat           string    `json:"timeFormat"`
	GatewayCount         int       `json:"gatewayCount"`
	CreatedUtc           time.Time `json:"createdUtc"`
}

func (l *Location) Url() *url.URL {
	return mustParse(fmt.Sprintf("%s/Locations/%d", BaseUrl, l.Id))
}

type Hilo struct {
	httpClient http.Client
	username   string
	password   string
	token      *Token
}

func NewHilo(username string, password string) *Hilo {
	return &Hilo{username: username, password: password}
}

func (h *Hilo) refreshToken() (*Token, error) {
	if h.token == nil || h.token.Expired() {
		return h.getToken()
	}
	return h.token, nil
}

func (h *Hilo) getToken() (*Token, error) {
	authUrl := "https://hilodirectoryb2c.b2clogin.com/hilodirectoryb2c.onmicrosoft.com/oauth2/v2.0/token?p=B2C_1A_B2C_1_PasswordFlow"
	resp, err := h.httpClient.PostForm(authUrl, url.Values{
		"grant_type":    []string{"password"},
		"scope":         []string{"openid 9870f087-25f8-43b6-9cad-d4b74ce512e1 offline_access"},
		"client_id":     []string{"9870f087-25f8-43b6-9cad-d4b74ce512e1"},
		"response_type": []string{"token id_token"},
		"username":      []string{h.username},
		"password":      []string{h.password},
	})
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to query hilo API HTTP status code %d", resp.StatusCode)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic("could not close body")
		}
	}(resp.Body)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read %s", err)
	}
	token := &Token{}
	err = json.Unmarshal(data, token)
	expiryTimer, err := strconv.Atoi(token.ExpiresIn)
	if err != nil {
		token.ExpiryDate = time.Now().Add(3600 * time.Second)
	} else {
		token.ExpiryDate = time.Now().Add(time.Second * time.Duration(expiryTimer*1000))
	}
	if err != nil {
		return nil, fmt.Errorf("could not decode %s", err)
	}
	return token, nil
}

func (h *Hilo) do(req *http.Request) (*http.Response, error) {
	var err error
	h.token, err = h.refreshToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", h.token.AccessToken))
	req.Header.Set("Ocp-Apim-Subscription-Key", SubscriptionKey)
	return h.httpClient.Do(req)
}

func mustParse(rawUrl string) *url.URL {
	pasedUrl, err := url.Parse(rawUrl)
	if err != nil {
		panic(err)
	}
	return pasedUrl
}

func (h *Hilo) Locations() ([]Location, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", BaseUrl, "Locations"), nil)
	if err != nil {
		panic(err)
	}
	resp, err := h.do(req)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var data []Location
	err = decoder.Decode(&data)
	return data, err
}

type Device struct {
	Id                  int         `json:"id"`
	AssetId             interface{} `json:"assetId"`
	Identifier          string      `json:"identifier"`
	GatewayId           int         `json:"gatewayId"`
	GatewayExternalId   string      `json:"gatewayExternalId"`
	Name                string      `json:"name"`
	Type                string      `json:"type"`
	GroupId             int         `json:"groupId"`
	Category            string      `json:"category"`
	Icon                interface{} `json:"icon"`
	LoadConnected       interface{} `json:"loadConnected"`
	ModelNumber         string      `json:"modelNumber"`
	LocationId          int         `json:"locationId"`
	Parameters          interface{} `json:"parameters"`
	ExternalGroup       string      `json:"externalGroup"`
	Provider            int         `json:"provider"`
	ProviderData        interface{} `json:"providerData"`
	Disconnected        bool        `json:"disconnected"`
	SupportedAttributes string      `json:"supportedAttributes"`
	SettableAttributes  string      `json:"settableAttributes"`
	SupportedParameters string      `json:"supportedParameters"`
}

func (d *Device) Url() *url.URL {
	location := Location{Id: d.LocationId}
	return mustParse(fmt.Sprintf("%s/Devices/%d", location.Url(), d.Id))
}

func (h *Hilo) Devices(location Location) ([]Device, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", location.Url(), "Devices"), nil)
	if err != nil {
		panic(err)
	}
	resp, err := h.do(req)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var data []Device
	err = decoder.Decode(&data)
	return data, err
}

type Gateway struct {
	OnlineStatus           string      `json:"onlineStatus"`
	LastStatusTimeUtc      time.Time   `json:"lastStatusTimeUtc"`
	ZigBeePairingActivated bool        `json:"zigBeePairingActivated"`
	Dsn                    string      `json:"dsn"`
	InstallationCode       string      `json:"installationCode"`
	SepMac                 string      `json:"sepMac"`
	FirmwareVersion        string      `json:"firmwareVersion"`
	LocalIp                interface{} `json:"localIp"`
	ZigBeeChannel          int         `json:"zigBeeChannel"`
}

func (h *Hilo) Gateways(location Location) ([]Gateway, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", location.Url(), "Gateways/Info"), nil)
	if err != nil {
		panic(err)
	}
	resp, err := h.do(req)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var data []Gateway
	err = decoder.Decode(&data)
	return data, err
}

type Attribute struct {
	DeviceId     int         `json:"deviceId"`
	LocationId   int         `json:"locationId"`
	TimeStampUTC time.Time   `json:"timeStampUTC"`
	Attribute    string      `json:"attribute"`
	Value        interface{} `json:"value"`
	ValueType    string      `json:"valueType"`
}

func (h *Hilo) DeviceAttributes(device Device) (map[string]Attribute, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", device.Url(), "Attributes"), nil)
	if err != nil {
		panic(err)
	}
	resp, err := h.do(req)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var data map[string]Attribute
	err = decoder.Decode(&data)
	return data, err
}
