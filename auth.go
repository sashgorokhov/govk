package govk

import (
	"strings"
	"github.com/PuerkitoBio/goquery"
	"github.com/levigross/grequests"
	"fmt"
	"strconv"
	"errors"
	"net/url"
	"io/ioutil"
	"bytes"
	"log"
)

const Login_url string = "https://oauth.vk.com/authorize"

type AuthInfo struct{
	Access_token string
	User_id      int
	Expires_in   int
}

type BufferedResponse struct {
	Response *grequests.Response
	Bytes    []byte
}

func CreateBufferedResponse(response *grequests.Response) (*BufferedResponse, error) {
	buf, err := ioutil.ReadAll(response.RawResponse.Body)
	if err != nil {
		return nil, err
	}
	return &BufferedResponse{Response:response, Bytes:buf}, nil
}

func (r *BufferedResponse) NewBuffer() *bytes.Buffer {
	return bytes.NewBuffer(r.Bytes)
}

func (r *BufferedResponse) GetDocument () (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(r.NewBuffer())
}

func Build_login_params(client_id int, scope *[]string) map[string]string {
	return map[string]string {
		"display" : "mobile",
		"redirect_uri" : "https://oauth.vk.com/blank.html",
		"response_type" : "token",
		"client_id" : strconv.Itoa(client_id),
		"scope" : strings.Join(*scope,","),
	}
}

func BuildLoginUrl(client_id int, scope *[]string) string {
	params := Build_login_params(client_id, scope)
	login_url, err := url.Parse(Login_url)
	if err != nil {
		return err.Error()
	}
	q := login_url.Query()
	for key, value := range params {
		q.Set(key, value)
	}

	return login_url.String() + "?" + q.Encode()

}

func process_form(form *goquery.Selection, query map[string]string, session *grequests.Session, last_response *BufferedResponse) (*BufferedResponse, error) {
	action, _ := form.Attr("action")
	if strings.HasPrefix(action, "/") {
		url := last_response.Response.RawResponse.Request.URL
		action = url.Scheme + "://" + url.Host + action
	}
	if query == nil {
		query = map[string]string{}
	}
	inputs := form.Find("input[type='hidden']")
	inputs.Each(func(i int, input *goquery.Selection) {
		name := input.AttrOr("name", "")
		value := input.AttrOr("value", "")
		if (name == "") || (value == "") {
			return
		}
		query[name] = value
	})
	response, err := session.Post(action, &grequests.RequestOptions{Data:query})
	if err != nil {
		return nil, err
	}
	return CreateBufferedResponse(response)
}

func auth_user(login, password string, doc *goquery.Document, session *grequests.Session, last_response *BufferedResponse) (*BufferedResponse, error) {
	form := doc.Find("form")
	buffered_response, err := process_form(form, map[string]string{"email": login, "pass": password}, session, last_response)
	if err != nil {
		return nil, err
	}
	doc, err = buffered_response.GetDocument()
	if err != nil {
		return nil, err
	}
	warning := doc.Find(".service_msg_warning")
	if warning.Length() > 0 {
		log.Println(warning.Children().Next().Text())
		return nil, errors.New(fmt.Sprintf("Authentication failure (probably invalid login or password): %s", warning.Next().Text()))
	}
	return buffered_response, nil
}

func process_two_factor_auth(auth_code string, doc *goquery.Document, session *grequests.Session, last_response *BufferedResponse) (*BufferedResponse, error) {
	form := doc.Find("form")
	buffered_response, err := process_form(form, map[string]string{"code": auth_code}, session, last_response)
	if err != nil {
		return nil, err
	}
	warning := doc.Find(".service_msg_warning")
	if warning.Length() > 0 {
		return nil, errors.New(fmt.Sprintf("Two-factor authentication failure: %s", warning.Next().Text()))
	}
	return buffered_response, nil
}

func give_access(doc *goquery.Document, session *grequests.Session, last_response *BufferedResponse) (*BufferedResponse, error) {
	form := doc.Find("form")
	return process_form(form, nil, session, last_response)
}

func Authenticate(login, password string, client_id int, scope *[]string, auth_code string) (*AuthInfo, error) {
	session := grequests.NewSession(nil)

	//Get initial login page
	response, err := session.Get(Login_url, &grequests.RequestOptions{Params: Build_login_params(client_id, scope)})
	if err != nil {
		return nil, err
	}
	buffered_response, err := CreateBufferedResponse(response)
	if err != nil {
		return nil, err
	}

	doc, err := buffered_response.GetDocument()
	if err != nil {
		return nil, err
	}

	//If password field is present, we need to give vk login and password
	s := doc.Find("input[name='pass']")
	if s.Length() > 0 {
		//If login/password incorrect, this will return an error
		buffered_response, err = auth_user(login, password, doc, session, buffered_response)
		if err != nil {
			return nil, err
		}
	}

	//Check if two-factor auth is enabled
	if buffered_response.Response.RawResponse.Request.URL.Path == "/login" {
		// Two-factor auth is enabled but no code was given
		if auth_code == "" {
			return nil, errors.New(fmt.Sprintf("Two-factor auth is enabled on account %s but no auth_code was given", login))
		}
		doc, err = buffered_response.GetDocument()
		if err != nil {
			return nil, err
		}
		// Process two-factor auth
		buffered_response, err = process_two_factor_auth(auth_code, doc, session, buffered_response)
		if err != nil {
			return nil, err
		}
	}

	//Check if user granted access for current app
	if buffered_response.Response.RawResponse.Request.URL.Path == "/authorize" {
		doc, err = buffered_response.GetDocument()
		if err != nil {
			return nil, err
		}
		// Just press the button
		buffered_response, err = give_access(doc, session, buffered_response)
		if err != nil {
			return nil, err
		}
	}

	if buffered_response.Response.RawResponse.Request.URL.Path != "/blank.html" {
		return nil, errors.New("Something went wrong")
	}
	
	query, err := url.ParseQuery(buffered_response.Response.RawResponse.Request.URL.Fragment)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error parsing query %s -- %s", buffered_response.Response.RawResponse.Request.URL, err))
	}
	user_id, _ := strconv.Atoi(query.Get("user_id"))
	expires_in, _ := strconv.Atoi(query.Get("expires_in"))
	return &AuthInfo{
		Access_token: query.Get("access_token"),
		User_id: user_id,
		Expires_in: expires_in,
	}, nil
	
}
