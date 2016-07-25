package govk

import (
	"strings"
	"github.com/PuerkitoBio/goquery"
	"github.com/levigross/grequests"
	"fmt"
	"strconv"
	"errors"
	"net/url"
)

const login_url string = "http://oauth.vk.com/authorize"

type AuthInfo struct{
	Access_token string
	User_id      int
	Expires_in   int
}

func build_login_params(client_id int, scope *[]string) map[string]string {
	return map[string]string {
		"display" : "mobile",
		"redirect_uri" : "http://oauth.vk.com/blank.html",
		"response_type" : "token",
		"client_id" : strconv.Itoa(client_id),
		"scope" : strings.Join(*scope,","),
	}
}

func process_form(form *goquery.Selection, query map[string]string, session *grequests.Session) (*grequests.Response, error) {
	action, _ := form.Attr("action")
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
	return session.Post(action, &grequests.RequestOptions{Params:query})
}

func auth_user(login, password string, doc *goquery.Document, session *grequests.Session) (*grequests.Response, error) {
	form := doc.Find("form")
	response, err := process_form(form, map[string]string{"email": login, "pass": password}, session)
	if err != nil {
		return nil, err
	}
	doc, err = goquery.NewDocumentFromResponse(response.RawResponse)
	if err != nil {
		return nil, err
	}
	warning := doc.Find(".service_msg_warning")
	if warning.Length() > 0 {
		return nil, errors.New(fmt.Sprintf("Invalid credentials: %s %s\n%s", login, password, warning.Next().Text()))
	}
	return response, nil
}

func give_access(doc *goquery.Document, session *grequests.Session) (*grequests.Response, error) {
	form := doc.Find("form")
	return process_form(form, nil, session)
}


func Authenticate(login, password string, client_id int, scope *[]string) (*AuthInfo, error) {
	session := grequests.NewSession(nil)

	response, err := session.Get(login_url, &grequests.RequestOptions{Params: build_login_params(client_id, scope)})
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromResponse(response.RawResponse)

	if err != nil {
		return nil, err
	}

	s := doc.Find("input[name='pass']")
	if s.Length() > 0 {
		response, err = auth_user(login, password, doc, session)
		if err != nil {
			return nil, err
		}
	}

	if response.RawResponse.Request.URL.Path != "/blank.html" {
		doc, err = goquery.NewDocumentFromResponse(response.RawResponse)
		if err != nil {
			return nil, err
		}
		response, err = give_access(doc, session)
		if err != nil {
			return nil, err
		}
	}


	if response.RawResponse.Request.URL.Path != "/blank.html" {
		return nil, errors.New("Something went wrong")
		
	}
	
	query, err := url.ParseQuery(response.RawResponse.Request.URL.Fragment)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error parsing query: %s\nUrl: %s", err, response.RawResponse.Request.URL))
	}
	user_id, _ := strconv.Atoi(query.Get("user_id"))
	expires_in, _ := strconv.Atoi(query.Get("expires_in"))
	return &AuthInfo{
		Access_token: query.Get("access_token"),
		User_id: user_id,
		Expires_in: expires_in,
	}, nil
	
}
