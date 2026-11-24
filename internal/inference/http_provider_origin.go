package inference

import (
	"errors"
	"net/url"
)

// ParseHTTPEndpointOrigin is shared by transport admission and administrative
// metadata reads. It performs no network access and grants no dial authority.
// In particular a changed owner file cannot expose userinfo, query secrets or
// path credentials through an otherwise read-only administration response.
func ParseHTTPEndpointOrigin(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || len(value) > 4096 || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || u.Path != "" && u.Path != "/" || u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New("HTTP inference endpoint must be an exact HTTP(S) origin")
	}
	return u, nil
}
