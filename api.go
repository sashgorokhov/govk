package govk

import (
	"github.com/Jeffail/gabs"
	"github.com/levigross/grequests"
)

const default_version string = "5.50"
const base_api_url string = "https://api.vk.com/method/"

type Api struct {
	Version string
	Access_token string
}


func NewApi(access_token string) (*Api) {
	return &Api{Version: default_version, Access_token: access_token}
}

func (a *Api) Call (method string, params map[string]string) (*gabs.Container, error) {
	if params == nil {
		params = map[string]string{}
	}
	params["v"] = a.Version
	if a.Access_token != "" {
		params["access_token"] = a.Access_token
	}

	request_options := grequests.RequestOptions{Params:params}
	url := base_api_url + method

	response, err := grequests.Get(url, &request_options)
	if err != nil {
		return nil, err
	}
	return gabs.ParseJSONBuffer(response.RawResponse.Body)
}