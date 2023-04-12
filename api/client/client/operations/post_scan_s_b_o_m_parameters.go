// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/openclarity/grype-server/api/client/models"
)

// NewPostScanSBOMParams creates a new PostScanSBOMParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPostScanSBOMParams() *PostScanSBOMParams {
	return &PostScanSBOMParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewPostScanSBOMParamsWithTimeout creates a new PostScanSBOMParams object
// with the ability to set a timeout on a request.
func NewPostScanSBOMParamsWithTimeout(timeout time.Duration) *PostScanSBOMParams {
	return &PostScanSBOMParams{
		timeout: timeout,
	}
}

// NewPostScanSBOMParamsWithContext creates a new PostScanSBOMParams object
// with the ability to set a context for a request.
func NewPostScanSBOMParamsWithContext(ctx context.Context) *PostScanSBOMParams {
	return &PostScanSBOMParams{
		Context: ctx,
	}
}

// NewPostScanSBOMParamsWithHTTPClient creates a new PostScanSBOMParams object
// with the ability to set a custom HTTPClient for a request.
func NewPostScanSBOMParamsWithHTTPClient(client *http.Client) *PostScanSBOMParams {
	return &PostScanSBOMParams{
		HTTPClient: client,
	}
}

/* PostScanSBOMParams contains all the parameters to send to the API endpoint
   for the post scan s b o m operation.

   Typically these are written to a http.Request.
*/
type PostScanSBOMParams struct {

	// Body.
	Body *models.SBOM

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the post scan s b o m params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PostScanSBOMParams) WithDefaults() *PostScanSBOMParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the post scan s b o m params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PostScanSBOMParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the post scan s b o m params
func (o *PostScanSBOMParams) WithTimeout(timeout time.Duration) *PostScanSBOMParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the post scan s b o m params
func (o *PostScanSBOMParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the post scan s b o m params
func (o *PostScanSBOMParams) WithContext(ctx context.Context) *PostScanSBOMParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the post scan s b o m params
func (o *PostScanSBOMParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the post scan s b o m params
func (o *PostScanSBOMParams) WithHTTPClient(client *http.Client) *PostScanSBOMParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the post scan s b o m params
func (o *PostScanSBOMParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the post scan s b o m params
func (o *PostScanSBOMParams) WithBody(body *models.SBOM) *PostScanSBOMParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the post scan s b o m params
func (o *PostScanSBOMParams) SetBody(body *models.SBOM) {
	o.Body = body
}

// WriteToRequest writes these params to a swagger request
func (o *PostScanSBOMParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error
	if o.Body != nil {
		if err := r.SetBodyParam(o.Body); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
