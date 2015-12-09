package dknet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultContentTypeV1_1        = "application/vnd.docker.plugins.v1.1+json"
	defaultImplementationManifest = `{"Implements": ["NetworkDriver"]}`
	defaultScope                  = `{"Scope": "local"}`
	emptyResponse                 = `{}`

	activatePath       = "/Plugin.Activate"
	capabilitiesPath   = "/NetworkDriver.GetCapabilities"
	createNetworkPath  = "/NetworkDriver.CreateNetwork"
	deleteNetworkPath  = "/NetworkDriver.DeleteNetwork"
	createEndpointPath = "/NetworkDriver.CreateEndpoint"
	endpointInfoPath   = "/NetworkDriver.EndpointOperInfo"
	deleteEndpointPath = "/NetworkDriver.DeleteEndpoint"
	joinPath           = "/NetworkDriver.Join"
	leavePath          = "/NetworkDriver.Leave"
	//discoverNewPath    = "/NetworkDriver.DiscoverNew"
	//discoverDeletePath = "/NetworkDriver.DiscoverDelete"
)

// Driver represent the interface a driver must fulfill.
type Driver interface {
	CreateNetwork(*CreateNetworkRequest) error
	DeleteNetwork(*DeleteNetworkRequest) error
	CreateEndpoint(*CreateEndpointRequest) error
	DeleteEndpoint(*DeleteEndpointRequest) error
	EndpointInfo(*InfoRequest) (*InfoResponse, error)
	Join(*JoinRequest) (*JoinResponse, error)
	Leave(*LeaveRequest) error
}

type CreateNetworkRequest struct {
	NetworkID string
	Options   map[string]interface{}
	IPv4Data  []*IPAMData
	IPv6Data  []*IPAMData
}

type IPAMData struct {
	AddressSpace string
	Pool         string
	Gateway      string
	AuxAddresses map[string]interface{}
}

type DeleteNetworkRequest struct {
	NetworkID string
}

type CreateEndpointRequest struct {
	NetworkID  string
	EndpointID string
	Interface  *EndpointInterface
	Options    map[string]interface{}
}

type EndpointInterface struct {
	Address     string
	AddressIPv6 string
	MacAddress  string
}

type DeleteEndpointRequest struct {
	NetworkID  string
	EndpointID string
}

type InterfaceName struct {
	SrcName   string
	DstPrefix string
}

type InfoRequest struct {
	NetworkID string
	EnpointID string
}

type InfoResponse struct {
	Value map[string]string
}

type JoinRequest struct {
	NetworkID  string
	EndpointID string
	SandboxKey string
	Options    map[string]interface{}
}

type StaticRoute struct {
	Destination string
	RouteType   int
	NextHop     string
}

type JoinResponse struct {
	Gateway       string
	InterfaceName InterfaceName
	StaticRoutes  []*StaticRoute
}

type LeaveRequest struct {
	NetworkID  string
	EndpointID string
	Options    map[string]interface{}
}

// Handler forwards requests and responses between the docker daemon and the plugin.
type Handler struct {
	driver Driver
	mux    *http.ServeMux
}

// NewHandler initializes the request handler with a driver implementation.
func NewHandler(driver Driver) *Handler {
	h := &Handler{driver, http.NewServeMux()}
	h.initMux()
	return h
}

func (h *Handler) initMux() {
	h.mux.HandleFunc(activatePath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", defaultContentTypeV1_1)
		fmt.Fprintln(w, defaultImplementationManifest)
	})

	h.mux.HandleFunc(capabilitiesPath, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, defaultScope)
	})

	h.mux.HandleFunc(createNetworkPath, func(w http.ResponseWriter, r *http.Request) {
		req := &CreateNetworkRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		err = h.driver.CreateNetwork(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		successResponse(w)
	})
	h.mux.HandleFunc(deleteNetworkPath, func(w http.ResponseWriter, r *http.Request) {
		req := &DeleteNetworkRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		err = h.driver.DeleteNetwork(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		successResponse(w)
	})
	h.mux.HandleFunc(createEndpointPath, func(w http.ResponseWriter, r *http.Request) {
		req := &CreateEndpointRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		err = h.driver.CreateEndpoint(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		successResponse(w)
	})
	h.mux.HandleFunc(deleteEndpointPath, func(w http.ResponseWriter, r *http.Request) {
		req := &DeleteEndpointRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		err = h.driver.DeleteEndpoint(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		successResponse(w)
	})
	h.mux.HandleFunc(endpointInfoPath, func(w http.ResponseWriter, r *http.Request) {
		req := &InfoRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		res, err := h.driver.EndpointInfo(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		objectResponse(w, res)
	})
	h.mux.HandleFunc(joinPath, func(w http.ResponseWriter, r *http.Request) {
		req := &JoinRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		res, err := h.driver.Join(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		objectResponse(w, res)
	})
	h.mux.HandleFunc(leavePath, func(w http.ResponseWriter, r *http.Request) {
		req := &LeaveRequest{}
		err := decodeRequest(r, req)
		if err != nil {
			badRequestResponse(w)
			return
		}
		err = h.driver.Leave(req)
		if err != nil {
			errorResponse(w, err)
			return
		}
		successResponse(w)
	})

}

// ServeTCP makes the handler to listen for request in a given TCP address.
// It also writes the spec file on the right directory for docker to read.
func (h *Handler) ServeTCP(pluginName, addr string) error {
	return h.listenAndServe("tcp", addr, pluginName)
}

// ServeUnix makes the handler to listen for requests in a unix socket.
// It also creates the socket file on the right directory for docker to read.
func (h *Handler) ServeUnix(systemGroup, addr string) error {
	return h.listenAndServe("unix", addr, systemGroup)
}

func (h *Handler) listenAndServe(proto, addr, group string) error {
	var (
		start = make(chan struct{})
		l     net.Listener
		err   error
		spec  string
	)

	server := http.Server{
		Addr:    addr,
		Handler: h.mux,
	}

	switch proto {
	case "tcp":
		l, spec, err = newTCPListener(group, addr, start)
	case "unix":
		l, spec, err = newUnixListener(addr, group, start)
	}

	if spec != "" {
		defer os.Remove(spec)
	}
	if err != nil {
		return err
	}

	close(start)
	return server.Serve(l)
}

func decodeRequest(r *http.Request, req interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		body, _ := ioutil.ReadAll(r.Body)
		log.Debugf("Error decoding request: %s", string(body))
		return err
	}
	return nil
}

func badRequestResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", defaultContentTypeV1_1)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"Err": "Failed to decode request",
	})
}

func errorResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", defaultContentTypeV1_1)
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"Err": err.Error(),
	})
}

func objectResponse(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", defaultContentTypeV1_1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(obj)
}

func successResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", defaultContentTypeV1_1)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, emptyResponse)
}
