/**
 */
package gorets

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const DEFAULT_TIMEOUT int = 300000

/** header values */
const (
	RETS_VERSION string = "RETS-Version"
	RETS_SESSION_ID string = "RETS-Session-ID"
	RETS_REQUEST_ID string = "RETS-Request-ID"
	USER_AGENT string = "User-Agent"
	RETS_UA_AUTH_HEADER string = "RETS-UA-Authorization"
	ACCEPT string = "Accept"
	ACCEPT_ENCODING string = "Accept-Encoding"
	CONTENT_ENCODING string = "Content-Encoding"
	DEFLATE_ENCODINGS string = "gzip,deflate"
	CONTENT_TYPE string = "Content-Type"
	WWW_AUTH string = "Www-Authenticate"
	WWW_AUTH_RESP string = "Authorization"

)

func NewSession(user, pw, userAgent, userAgentPw string) *Session {
	var session Session
	session.Username = user
	session.Password = pw
	session.Version = "RETS/1.5"
	session.UserAgent = userAgent
	session.HttpMethod = "GET"
	session.Accept = "*/*"
	session.Client = &http.Client{}
	return &session
}

type Session struct {
	Username,Password string
	UserAgentPassword string
	HttpMethod string
	Version string // "Threewide/1.5"

	Accept string
	UserAgent string

	Client *http.Client

	Urls *CapabilityUrls
}


type RetsResponse struct {
	ReplyCode int
	ReplyText string
}

type CapabilityUrls struct {
	Response RetsResponse

	MemberName, User, Broker, MetadataVersion, MinMetadataVersion string
	OfficeList []string
	TimeoutSeconds int
	/** urls for web calls */
	Login,Action,Search,Get,GetObject,Logout,GetMetadata,ChangePassword string
}

func (c *CapabilityUrls) parse(response []byte) (error){
	type Rets struct {
		XMLName xml.Name `xml:"RETS"`
		ReplyCode int `xml:"ReplyCode,attr"`
		ReplyText string `xml:"ReplyText,attr"`
		Response string `xml:"RETS-RESPONSE"`
	}

	rets := Rets{}
	decoder := xml.NewDecoder(bytes.NewBuffer(response))
	decoder.Strict = false
	decoder.AutoClose = append(decoder.AutoClose,"RETS")
	err := decoder.Decode(&rets)
	if err != nil {
		return err
	}
	// TODO set the values that we parse from the body
	fmt.Println(rets.ReplyCode, rets.ReplyText, rets.Response)
	return nil
}

func (s *Session) Login(loginUrl string) (*CapabilityUrls, error) {
	req, err := http.NewRequest(s.HttpMethod, loginUrl, nil)

	if err != nil {
		fmt.Println(err)
		// handle error
	}

	// TODO do all of this per request
	req.Header.Add(USER_AGENT, s.UserAgent)
	req.Header.Add(RETS_VERSION, s.Version)
	req.Header.Add(ACCEPT, s.Accept)

	resp, err := s.Client.Do(req)

	if err != nil {
		return nil, err
	}

	// TODO set digest auth up as a handler
	if resp.StatusCode == 401 {
		challenge := resp.Header.Get(WWW_AUTH)
		if !strings.HasPrefix(strings.ToLower(challenge), "digest") {
			return nil, errors.New("unknown authentication challenge: "+challenge)
		}
		req.Header.Add(WWW_AUTH_RESP, DigestResponse(challenge, s.Username, s.Password, req.Method, req.URL.Path))
		resp, err = s.Client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 401 {
			return nil, errors.New("authentication failed: "+s.Username)
		}
	}
	if s.UserAgentPassword != "" {
		requestId := resp.Header.Get(RETS_REQUEST_ID)
		sessionId := ""// TODO resp.Cookies().Get(RETS_SESSION_ID)
		uaAuthHeader := CalculateUaAuthHeader(s.UserAgent, s.UserAgentPassword, requestId, sessionId, s.Version)
		req.Header.Add(RETS_UA_AUTH_HEADER, uaAuthHeader)
	}

	capabilities, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var urls = &CapabilityUrls{}
	err = urls.parse(capabilities)
	if err != nil {
		return nil, errors.New("unable to parse capabilites response: "+string(capabilities))
	}
	// TODO do i really want to return _and_ set this value on myself?
	s.Urls = urls
	return urls, nil
}

