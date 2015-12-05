package tor

import (
	"fmt"
)

// EndpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type EndpointNotFoundError string

func (enfe EndpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", string(enfe))
}

// NotFound denotes the type of this error
func (enfe EndpointNotFoundError) NotFound() {}

// ErrInvalidEndpointConfig error is returned when a endpoint create is attempted with an invalid endpoint configuration.
type ErrInvalidEndpointConfig struct{}

func (eiec *ErrInvalidEndpointConfig) Error() string {
	return "trying to create an endpoint with an invalid endpoint configuration"
}

// BadRequest denotes the type of this error
func (eiec *ErrInvalidEndpointConfig) BadRequest() {}

// InvalidEndpointIDError is returned when the passed
// endpoint id is not valid.
type InvalidEndpointIDError string

func (ieie InvalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id: %s", string(ieie))
}

// BadRequest denotes the type of this error
func (ieie InvalidEndpointIDError) BadRequest() {}

// ErrUnsupportedAddressType is returned when the specified address type is not supported.
type ErrUnsupportedAddressType string

func (uat ErrUnsupportedAddressType) Error() string {
	return fmt.Sprintf("unsupported address type: %s", string(uat))
}

// BadRequest denotes the type of this error
func (uat ErrUnsupportedAddressType) BadRequest() {}
