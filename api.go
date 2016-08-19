// govk provides go language interface to vkontakte social network api
package govk

import (
	"bytes"
	"encoding/json"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"github.com/levigross/grequests"
	"github.com/x-cray/logrus-prefixed-formatter"
	"io/ioutil"
)

const default_version string = "5.50"
const base_api_url string = "https://api.vk.com/method/"

// ApiLogger used by Api instance and logs INFO message before and after every request with request params and response body
var ApiLogger = (&logrus.Logger{
	Level:     logrus.InfoLevel,
	Formatter: new(prefixed.TextFormatter),
	Hooks:     make(logrus.LevelHooks),
	Out:       ioutil.Discard,
}).WithField("prefix", "govk.api")

// Api is a general entity of this package.
type Api struct {
	Version      string
	Access_token string
}

// NewApi creates a new Api instance with default version and provided access token
func NewApi(access_token string) *Api {
	return &Api{Version: default_version, Access_token: access_token}
}

// prepare_params creates grequests.RequestOptions from given params map, and inserts version and access token fields.
func (a *Api) prepare_params(params map[string]string) *grequests.RequestOptions {
	if params == nil {
		params = make(map[string]string)
	}
	params["v"] = a.Version
	if a.Access_token != "" {
		params["access_token"] = a.Access_token
	}
	return &grequests.RequestOptions{Params: params}
}

// RawRequest is a base method that executes an api request on vkontakte api and returns grequests.Response
func (a *Api) RawRequest(method string, params map[string]string) (*grequests.Response, error) {
	url := base_api_url + method
	prepared_params := a.prepare_params(params)
	request_logger := ApiLogger.WithFields(logrus.Fields{
		"url":    url,
		"params": prepared_params.Params,
	})
	request_logger.Infoln("Making request")
	response, err := grequests.Get(url, prepared_params)
	request_logger.WithFields(logrus.Fields{
		"status":   response.StatusCode,
		"response": response.String(),
	}).Infoln("Request made")
	return response, err
}

// AbstractRequest executes an api request and returns gabs.Container instance.
// Useful if you don't want to deal with structs.
func (a *Api) AbstractRequest(method string, params map[string]string) (*gabs.Container, error) {
	response, err := a.RawRequest(method, params)
	if err != nil {
		return nil, err
	}
	return gabs.ParseJSONBuffer(bytes.NewReader(response.Bytes()))
}

// StructRequest executes an api request and decodes json response, putting results in user_struct.
// If response contains an "error" key on top of the response, an instance of ResponseError is returned.
func (a *Api) StructRequest(method string, params map[string]string, user_struct interface{}) error {
	containter, err := a.AbstractRequest(method, params)
	if err != nil {
		return err
	}
	if containter.ExistsP("error") {
		var error_struct ErrorResponseStruct
		err = json.NewDecoder(bytes.NewReader(containter.Bytes())).Decode(&error_struct)
		if err != nil {
			return err
		}
		return ResponseError{ErrorStruct: error_struct}
	} else {
		return json.NewDecoder(bytes.NewReader(containter.Bytes())).Decode(user_struct)
	}
}
