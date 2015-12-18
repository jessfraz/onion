package dknet

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

type TestDriver struct {
	Driver
}

func (t *TestDriver) CreateNetwork(r *CreateNetworkRequest) error {
	return nil
}

func (t *TestDriver) DeleteNetwork(r *DeleteNetworkRequest) error {
	return nil
}

func (t *TestDriver) CreateEndpoint(r *CreateEndpointRequest) error {
	return nil
}

func (t *TestDriver) DeleteEndpoint(r *DeleteEndpointRequest) error {
	return nil
}

func (t *TestDriver) Join(r *JoinRequest) (*JoinResponse, error) {
	return &JoinResponse{}, nil
}

func (t *TestDriver) Leave(r *LeaveRequest) error {
	return nil
}

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MySuite struct {
	h *Handler
}

var _ = Suite(&MySuite{})

func (s *MySuite) SetUpSuite(c *C) {
	d := &TestDriver{}
	s.h = NewHandler(d)
	go s.h.ServeTCP("test", ":8080")
}

func (s *MySuite) TestActivate(c *C) {
	response, err := http.Get("http://localhost:8080/Plugin.Activate")
	if err != nil {
		c.Fatal(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	c.Assert(string(body), Equals, defaultImplementationManifest+"\n")

}

func (s *MySuite) TestCapabilitiesExchange(c *C) {
	response, err := http.Get("http://localhost:8080/NetworkDriver.GetCapabilities")
	if err != nil {
		c.Fatal(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	c.Assert(string(body), Equals, defaultScope+"\n")

}

func (s *MySuite) TestCreateNetwork400(c *C) {
	response, err := http.Post("http://localhost:8080/NetworkDriver.CreateNetwork",
		defaultContentTypeV1_1,
		nil)
	if err != nil {
		c.Fatal(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	c.Assert(response.StatusCode, Equals, http.StatusBadRequest)
	c.Assert(string(body), Equals, `{"Err":"Failed to decode request"}`+"\n")

}

func (s *MySuite) TestCreateNetwork200(c *C) {
	request := `{"NetworkID":"d76cfa51738e8a12c5eca71ee69e9d65010a4b48eaad74adab439be7e61b9aaf","Options":{"com.docker.network.generic":{}},"IPv4Data":[{"AddressSpace":"","Gateway":"172.18.0.1/16","Pool":"172.18.0.0/16"}],"IPv6Data":[]}`

	response, err := http.Post("http://localhost:8080/NetworkDriver.CreateNetwork",
		defaultContentTypeV1_1,
		strings.NewReader(request),
	)
	if err != nil {
		c.Fatal(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "")

}
