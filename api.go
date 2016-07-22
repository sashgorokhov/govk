package govk

import (
	"github.com/Jeffail/gabs"
	"github.com/levigross/grequests"
)

const DEFAULT_VERSION string = "5.50"
const BASE_API_URL string = "https://api.vk.com/method/"

type Api struct {
	Version string
	Access_token string
}


func NewApi(access_token string) (*Api) {
	return &Api{Version: DEFAULT_VERSION, Access_token: access_token}
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
	url := BASE_API_URL + method

	response, err := grequests.Get(url, &request_options)
	if err != nil {
		return nil, err
	}
	return gabs.ParseJSONBuffer(response.RawResponse.Body)
}