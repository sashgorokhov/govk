package govk

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/Sirupsen/logrus"
	"github.com/levigross/grequests"
	"github.com/x-cray/logrus-prefixed-formatter"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AuthLogger used during authenticating procedure.
var AuthLogger = (&logrus.Logger{
	Level:     logrus.InfoLevel,
	Formatter: new(prefixed.TextFormatter),
	Hooks:     make(logrus.LevelHooks),
	Out:       ioutil.Discard,
}).WithField("prefix", "govk.auth")

const login_url string = "https://oauth.vk.com/authorize"

// AuthInfo is a struct that holds authentication results like access token, user id and expiration time
type AuthInfo struct {
	Access_token string
	User_id      int
	Expires_in   int
	Expires_at   time.Time
}

// bufferedResponse for internal use
type bufferedResponse struct {
	Response *grequests.Response
	Bytes    []byte
}

func createBufferedResponse(response *grequests.Response) (*bufferedResponse, error) {
	buf, err := ioutil.ReadAll(response.RawResponse.Body)
	if err != nil {
		return nil, err
	}
	return &bufferedResponse{Response: response, Bytes: buf}, nil
}

func (r *bufferedResponse) NewBuffer() *bytes.Buffer {
	return bytes.NewBuffer(r.Bytes)
}

func (r *bufferedResponse) GetDocument() (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(r.NewBuffer())
}

// BuildLoginParams returns a map with data for initial login page
func BuildLoginParams(client_id int, scope *[]string) map[string]string {
	return map[string]string{
		"display":       "mobile",
		"redirect_uri":  "https://oauth.vk.com/blank.html",
		"response_type": "token",
		"client_id":     strconv.Itoa(client_id),
		"scope":         strings.Join(*scope, ","),
	}
}

// BuildLoginUrl returns an url for initial login page
func BuildLoginUrl(client_id int, scope *[]string) string {
	params := BuildLoginParams(client_id, scope)
	login_url, err := url.Parse(login_url)
	if err != nil {
		return err.Error()
	}
	q := login_url.Query()
	for key, value := range params {
		q.Set(key, value)
	}

	return login_url.String() + "?" + q.Encode()

}

func hide_password(query map[string]string) map[string]string {
	new_query := make(map[string]string)
	for k, v := range query {
		new_query[k] = v
	}
	value, found := new_query["pass"]
	if found {
		new_query["pass"] = strings.Repeat("*", len(value))
	}
	return new_query
}

// process_form just POST a given form with all data
func process_form(form *goquery.Selection, query map[string]string, session *grequests.Session, last_response *bufferedResponse) (*bufferedResponse, error) {
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
	AuthLogger.WithFields(logrus.Fields{
		"query":  hide_password(query),
		"action": action,
	}).Debugln("Processing form")
	response, err := session.Post(action, &grequests.RequestOptions{Data: query})
	if err != nil {
		return nil, err
	}
	return createBufferedResponse(response)
}

// auth_user fills users login and password in form and executes it.
func auth_user(login, password string, doc *goquery.Document, session *grequests.Session, last_response *bufferedResponse) (*bufferedResponse, error) {
	auth_user_logger := AuthLogger.WithFields(logrus.Fields{
		"login": login,
	})
	form := doc.Find("form")
	buffered_response, err := process_form(form, map[string]string{"email": login, "pass": password}, session, last_response)
	if err != nil {
		auth_user_logger.Errorln("Error while processing login form")
		return nil, err
	}
	doc, err = buffered_response.GetDocument()
	if err != nil {
		auth_user_logger.Errorln("Error while getting login response document")
		return nil, err
	}
	warning := doc.Find(".service_msg_warning")
	if warning.Length() > 0 {
		auth_user_logger.Warningln(warning.Next().Text())
		return nil, errors.New(fmt.Sprintf("Authentication failure (probably invalid login or password): %s", warning.Next().Text()))
	}
	return buffered_response, nil
}

// process_two_factor_auth passes two-factor auth with auth_code
func process_two_factor_auth(auth_code string, doc *goquery.Document, session *grequests.Session, last_response *bufferedResponse) (*bufferedResponse, error) {
	logger := AuthLogger.WithField("auth_code", auth_code)
	form := doc.Find("form")
	buffered_response, err := process_form(form, map[string]string{"code": auth_code}, session, last_response)
	if err != nil {
		logger.Errorln("Error while processing two-factor auth form")
		return nil, err
	}
	warning := doc.Find(".service_msg_warning")
	if warning.Length() > 0 {
		logger.Warningln(warning.Next().Text())
		return nil, errors.New(fmt.Sprintf("Two-factor authentication failure: %s", warning.Next().Text()))
	}
	return buffered_response, nil
}

// give_access confirmes requested permissions
func give_access(doc *goquery.Document, session *grequests.Session, last_response *bufferedResponse) (*bufferedResponse, error) {
	form := doc.Find("form")
	return process_form(form, nil, session, last_response)
}

// Authenticate user by login and password, optionally passing two-factor auth with auth_code (if it is not enabled
// on account, pass "" in auth_code. Return AuthInfo structure. And error, of course.
func Authenticate(login, password string, client_id int, scope *[]string, auth_code string) (*AuthInfo, error) {
	AuthLogger.WithFields(logrus.Fields{
		"login":     login,
		"client_id": client_id,
		"scope":     *scope,
		"auth_code": auth_code,
	}).Infoln("Authenticating")
	session := grequests.NewSession(nil)

	//Get initial login page
	login_params := BuildLoginParams(client_id, scope)

	authentication_logger := AuthLogger.WithFields(logrus.Fields{
		"login_params": login_params,
		"login_url":    login_url,
	})
	authentication_logger.Infoln("Getting initial login page")

	response, err := session.Get(login_url, &grequests.RequestOptions{Params: login_params})
	if err != nil {
		authentication_logger.Errorln("Error while getting initial login page")
		return nil, err
	}

	if !response.Ok {
		authentication_logger.WithFields(logrus.Fields{
			"status": response.StatusCode,
			"ok":     response.Ok,
		}).Warningln("Initial login page response status is not Ok")
		return nil, errors.New("Initial login page response status is not Ok")
	}

	buffered_response, err := createBufferedResponse(response)
	if err != nil {
		authentication_logger.Errorln("Error while creating buffered response from initial login page")
		return nil, err
	}

	doc, err := buffered_response.GetDocument()
	if err != nil {
		authentication_logger.Errorln("Error while creating document from initial login page")
		return nil, err
	}

	login_logger := AuthLogger.WithField("login", login)

	//If password field is present, we need to give vk login and password
	s := doc.Find("input[name='pass']")
	if s.Length() > 0 {
		login_logger.Infoln("Found password input field")
		//If login/password incorrect, this will return an error
		buffered_response, err = auth_user(login, password, doc, session, buffered_response)
		if err != nil {
			login_logger.Errorln("Error authenticating user")
			return nil, err
		}
		login_logger.Infoln("User authenticated")
	}

	//Check if two-factor auth is enabled
	if buffered_response.Response.RawResponse.Request.URL.Path == "/login" {
		auth_code_logger := login_logger.WithField("auth_code", auth_code)

		auth_code_logger.Infoln("Seems like two-factor auth is enabled on account")
		// Two-factor auth is enabled but no code was given
		if auth_code == "" {
			auth_code_logger.Warningln("Two-factor auth is enabled on account, but no auth_code was given")
			return nil, errors.New(fmt.Sprintf("Two-factor auth is enabled on account %s but no auth_code was given", login))
		}
		doc, err = buffered_response.GetDocument()
		if err != nil {
			auth_code_logger.Errorln("Error getting two-factor page document")
			return nil, err
		}
		// Process two-factor auth
		buffered_response, err = process_two_factor_auth(auth_code, doc, session, buffered_response)
		if err != nil {
			auth_code_logger.Errorln("Error processing two-factor auth")
			return nil, err
		}
		auth_code_logger.Infoln("Passed two-factor auth")
	}

	//Check if user granted access for current app
	if buffered_response.Response.RawResponse.Request.URL.Path == "/authorize" {
		login_logger.Infoln("Permissions approve required")
		doc, err = buffered_response.GetDocument()
		if err != nil {
			login_logger.Errorln("Error while creating document for permissions approval")
			return nil, err
		}
		// Just press the button
		buffered_response, err = give_access(doc, session, buffered_response)
		if err != nil {
			login_logger.Errorln("Error while permissions approval")
			return nil, err
		}
		login_logger.Infoln("Permissions approved")
	}

	if buffered_response.Response.RawResponse.Request.URL.Path != "/blank.html" {
		url := buffered_response.Response.RawResponse.Request.URL.String()
		login_logger.WithFields(logrus.Fields{
			"url": url,
		}).Warningln("Unexpected page url (expected /blank.html)")
		return nil, errors.New(fmt.Sprintf("Unexpected page url (expected /blank.html): %s", url))
	}

	query, err := url.ParseQuery(buffered_response.Response.RawResponse.Request.URL.Fragment)
	if err != nil {
		login_logger.WithFields(logrus.Fields{
			"url": buffered_response.Response.RawResponse.Request.URL.String(),
			"err": err,
		}).Warningln("Cant parse url query")
		return nil, errors.New(fmt.Sprintf("Error parsing query %s -- %s", buffered_response.Response.RawResponse.Request.URL, err))
	}
	user_id, _ := strconv.Atoi(query.Get("user_id"))
	expires_in, _ := strconv.Atoi(query.Get("expires_in"))
	expires_at := time.Now().Add(time.Duration(expires_in) * time.Second)
	return &AuthInfo{
		Access_token: query.Get("access_token"),
		User_id:      user_id,
		Expires_in:   expires_in,
		Expires_at:   expires_at,
	}, nil

}
