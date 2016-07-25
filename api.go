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


func (a *Api) prepare_params (params map[string]string) *grequests.RequestOptions {
	if params == nil {
		params = make(map[string]string)
	}
	params["v"] = a.Version
	if a.Access_token != "" {
		params["access_token"] = a.Access_token
	}

	return &grequests.RequestOptions{Params:params}
}

func (a *Api) RawRequest(method string, params map[string]string) (*grequests.Response, error) {
	url := base_api_url + method
	return grequests.Get(url, a.prepare_params(params))
}


func (a *Api) AbstractRequest (method string, params map[string]string) (*gabs.Container, error) {
	response, err := a.RawRequest(method, params)
	if err != nil {
		return nil, err
	}
	return gabs.ParseJSONBuffer(response.RawResponse.Body)
}


func (a *Api) StructRequest (method string, params map[string]string, user_struct interface{}) (error) {
	response, err := a.RawRequest(method, params)
	if err != nil {
		return err
	}
	response.JSON(user_struct)
	return nil
}
