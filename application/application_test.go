package application

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/wissance/Ferrum/config"
	"github.com/wissance/Ferrum/data"
	"github.com/wissance/Ferrum/dto"
	"github.com/wissance/Ferrum/errors"
	"github.com/wissance/stringFormatter"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const testAccessTokenExpiration = 10
const testRefreshTokenExpiration = 5
const testRealm1 = "testrealm1"
const testClient1 = "testclient1"
const testClient1Secret = "fb6Z4RsOadVycQoeQiN57xpu8w8wplYz"

var testKey = []byte("qwerty1234567890")
var testServerData = data.ServerData{
	Realms: []data.Realm{
		{Name: testRealm1, TokenExpiration: testAccessTokenExpiration, RefreshTokenExpiration: testRefreshTokenExpiration,
			Clients: []data.Client{
				{Name: testClient1, Type: data.Confidential, Auth: data.Authentication{Type: data.ClientIdAndSecrets,
					Value: testClient1Secret}},
			}, Users: []interface{}{
				map[string]interface{}{"info": map[string]interface{}{"sub": "667ff6a7-3f6b-449b-a217-6fc5d9ac0723",
					"name": "vano", "preferred_username": "vano",
					"given_name": "vano ivanov", "family_name": "ivanov", "email_verified": true},
					"credentials": map[string]interface{}{"password": "1234567890"}},
			}},
	},
}

// todo (UMV): add here Path combine instead of  //
var loggingConfig = config.LoggingConfig{Level: "info", Appenders: []config.AppenderConfig{{Level: "info", Type: config.Console}}}
var httpAppConfig = config.AppConfig{ServerCfg: config.ServerConfig{Schema: config.HTTP, Address: "127.0.0.1", Port: 8284}, Logging: loggingConfig}
var httpsAppConfig = config.AppConfig{ServerCfg: config.ServerConfig{Schema: config.HTTPS, Address: "127.0.0.1", Port: 8672,
	Security: config.SecurityConfig{KeyFile: "../certs/server.key", CertificateFile: "../certs/server.crt"}}, Logging: loggingConfig}

// todo(UMV): take from config
func TestApplicationOnHttp(t *testing.T) {
	testRunCommonTestCycleImpl(t, &httpAppConfig, "http://127.0.0.1:8284")
}

// todo(UMV): take from config
func TestApplicationOnHttps(t *testing.T) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	testRunCommonTestCycleImpl(t, &httpsAppConfig, "https://127.0.0.1:8672")
}

func testRunCommonTestCycleImpl(t *testing.T, appConfig *config.AppConfig, baseUrl string) {
	app := CreateAppWithData(appConfig, &testServerData, testKey)
	res, err := app.Init()
	assert.True(t, res)
	assert.Nil(t, err)

	res, err = app.Start()
	assert.True(t, res)
	assert.Nil(t, err)
	realm := testRealm1
	username := "vano"
	// 1. Issue new valid token and get userInfo
	response := issueNewToken(t, baseUrl, realm, testClient1, testClient1Secret, username, "1234567890")
	token := getDataFromResponse[dto.Token](t, response)
	assert.True(t, len(token.AccessToken) > 0)
	assert.True(t, len(token.RefreshToken) > 0)
	// check token by query username
	userInfo := getUserInfo(t, baseUrl, realm, token.AccessToken, "200 OK")
	assert.True(t, len(userInfo) > 0)
	assert.Equal(t, username, userInfo["preferred_username"])

	// 2. Introspect valid token
	// todo(UMV): add Introspect result check
	tokenIntResult := checkIntrospectToken(t, baseUrl, realm, token.AccessToken, testClient1, testClient1Secret, "200 OK")
	active, ok := tokenIntResult["active"]
	assert.True(t, ok)
	assert.True(t, active.(bool))
	delay := 3
	time.Sleep(time.Second * time.Duration(delay))
	// 3. Refresh token successfully
	response = refreshToken(t, baseUrl, realm, testClient1, testClient1Secret, token.RefreshToken)
	assert.Equal(t, response.Status, "200 OK")
	token = getDataFromResponse[dto.Token](t, response)
	time.Sleep(time.Second * time.Duration(testAccessTokenExpiration-delay+1))
	checkIntrospectToken(t, baseUrl, realm, token.AccessToken, testClient1, testClient1Secret, "200 OK")
	// 4. Use wrong params to  token introspection and check status
	checkIntrospectToken(t, baseUrl, realm, token.AccessToken, "wrongClientId", testClient1Secret, "401 Unauthorized")
	checkIntrospectToken(t, baseUrl, realm, token.AccessToken, testClient1, "wrongSecret", "401 Unauthorized")
	checkIntrospectToken(t, baseUrl, realm, "wrongToken", testClient1, testClient1Secret, "401 Unauthorized")

	// 5. Expire token by timeout and got 401 (Unauthorized) status
	time.Sleep(time.Second * time.Duration(testAccessTokenExpiration))
	userInfo = getUserInfo(t, baseUrl, realm, token.AccessToken, "401 Unauthorized")
	// todo(UMV): this one looking strange because token expired and we expect here 200 as status
	tokenIntResult = checkIntrospectToken(t, baseUrl, realm, token.AccessToken, testClient1, testClient1Secret, "200 OK")
	active, ok = tokenIntResult["active"]
	assert.True(t, ok == false || active == nil || active.(bool) == false)
	// 6. Attempt to get new tokens with wrong credentials
	response = issueNewToken(t, baseUrl, realm, "unknownClient", testClient1Secret, username, "1234567890")
	errResp := getDataFromResponse[dto.ErrorDetails](t, response)
	assert.Equal(t, errors.InvalidClientMsg, errResp.Msg)
	// try with bad user credentials
	response = issueNewToken(t, baseUrl, realm, testClient1, testClient1Secret, username, "wrongPass!!!")
	errResp = getDataFromResponse[dto.ErrorDetails](t, response)
	assert.Equal(t, errors.InvalidUserCredentialsMsg, errResp.Msg)

	// 6. Issue new valid token and wait refresh expiration and check
	response = issueNewToken(t, baseUrl, realm, testClient1, testClient1Secret, username, "1234567890")
	assert.Equal(t, response.Status, "200 OK")
	token = getDataFromResponse[dto.Token](t, response)
	time.Sleep(time.Second * time.Duration(testRefreshTokenExpiration+2))
	response = refreshToken(t, baseUrl, realm, testClient1, testClient1Secret, token.RefreshToken)
	assert.Equal(t, response.Status, "400 Bad Request")
	// but still possible to get userInfo with accessToken
	userInfo = getUserInfo(t, baseUrl, realm, token.AccessToken, "200 OK")
	assert.True(t, len(userInfo) > 0)
	assert.Equal(t, username, userInfo["preferred_username"])

	res, err = app.Stop()
	assert.True(t, res)
	assert.Nil(t, err)
}

func issueNewToken(t *testing.T, baseUrl string, realm string, clientId string, clientSecret string,
	userName string, password string) *http.Response {
	tokenUrlTemplate := "{0}/auth/realms/{1}/protocol/openid-connect/token"
	tokenUrl := stringFormatter.Format(tokenUrlTemplate, baseUrl, realm)
	getTokenData := url.Values{}
	getTokenData.Set("client_id", clientId)
	getTokenData.Set("client_secret", clientSecret)
	getTokenData.Set("scope", "profile")
	getTokenData.Set("grant_type", "password")
	getTokenData.Set("username", userName)
	getTokenData.Set("password", password)
	response, err := http.PostForm(tokenUrl, getTokenData)
	assert.Nil(t, err)
	return response
}

func refreshToken(t *testing.T, baseUrl string, realm string, clientId string, clientSecret string,
	refreshToken string) *http.Response {
	tokenUrlTemplate := "{0}/auth/realms/{1}/protocol/openid-connect/token"
	tokenUrl := stringFormatter.Format(tokenUrlTemplate, baseUrl, realm)
	getTokenData := url.Values{}
	getTokenData.Set("client_id", clientId)
	getTokenData.Set("client_secret", clientSecret)
	getTokenData.Set("scope", "profile")
	getTokenData.Set("grant_type", "refresh_token")
	getTokenData.Set("refresh_token", refreshToken)
	response, err := http.PostForm(tokenUrl, getTokenData)
	assert.Nil(t, err)
	return response
}

func getDataFromResponse[TR dto.Token | dto.ErrorDetails](t *testing.T, response *http.Response) TR {
	responseBody, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	var result TR
	err = json.Unmarshal(responseBody, &result)
	assert.Nil(t, err)
	return result
}

func getUserInfo(t *testing.T, baseUrl string, realm string, token string, expectedStatus string) map[string]interface{} {
	userInfoUrlTemplate := "{0}/auth/realms/{1}/protocol/openid-connect/userinfo/"
	userInfoUrl := stringFormatter.Format(userInfoUrlTemplate, baseUrl, realm)
	client := http.Client{}
	request, err := http.NewRequest("GET", userInfoUrl, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	assert.Nil(t, err)
	response, err := client.Do(request)
	assert.Equal(t, expectedStatus, response.Status)
	assert.Nil(t, err)
	responseBody, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	assert.Nil(t, err)
	return result
}
func checkIntrospectToken(t *testing.T, baseUrl string, realm string, token string, clientId string, clientSecret string, expectedStatus string) map[string]interface{} {
	urlTemplate := "{0}/auth/realms/{1}/protocol/openid-connect/token/introspect"
	reqUrl := stringFormatter.Format(urlTemplate, baseUrl, realm)
	client := http.Client{}
	formData := url.Values{}
	formData.Set("token_type_hint", "requesting_party_token")
	formData.Set("token", token)
	request, err := http.NewRequest("POST", reqUrl, strings.NewReader(formData.Encode()))
	assert.NoError(t, err)
	httpBasicAuth := base64.StdEncoding.EncodeToString([]byte(clientId + ":" + clientSecret))
	request.Header.Set("Authorization", "Basic "+httpBasicAuth)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := client.Do(request)
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, response.Status)
	responseBody, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	assert.Nil(t, err)
	return result
}
