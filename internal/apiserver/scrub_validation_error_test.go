package apiserver_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/apiserver"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

var _ = Describe("scrubValidationError", func() {

	// Branch 1: RequestError with nested SchemaError
	It("extracts SchemaError reason with parameter prefix", func() {
		schemaErr := &openapi3.SchemaError{
			Value:  1001,
			Reason: "must be <= 1000",
		}
		reqErr := &openapi3filter.RequestError{
			Parameter: &openapi3.Parameter{Name: "max_page_size", In: "query"},
			Err:       schemaErr,
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal(`parameter "max_page_size" in query: must be <= 1000`))
	})

	// Branch 1 variant: SchemaError without parameter context
	It("returns SchemaError reason without prefix when no parameter", func() {
		schemaErr := &openapi3.SchemaError{
			Value:  nil,
			Reason: "value is required",
		}
		reqErr := &openapi3filter.RequestError{
			RequestBody: nil,
			Err:         schemaErr,
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal("value is required"))
	})

	// Branch 1 variant: SchemaError with request body prefix
	It("extracts SchemaError reason with request body prefix", func() {
		schemaErr := &openapi3.SchemaError{
			Value:  nil,
			Reason: "property \"image\" is missing",
		}
		reqErr := &openapi3filter.RequestError{
			RequestBody: &openapi3.RequestBody{},
			Err:         schemaErr,
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal(`request body: property "image" is missing`))
	})

	// Branch 2: RequestError with Reason only (no SchemaError)
	It("falls back to RequestError.Reason when no SchemaError", func() {
		reqErr := &openapi3filter.RequestError{
			Parameter: &openapi3.Parameter{Name: "container_id", In: "path"},
			Reason:    "value is not valid",
			Err:       errors.New("some internal error"),
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal(`parameter "container_id" in path: value is not valid`))
	})

	// Branch 2 variant: RequestError with Reason but no parameter
	It("returns RequestError.Reason without prefix when no parameter", func() {
		reqErr := &openapi3filter.RequestError{
			Reason: "request body has an error",
			Err:    errors.New("some internal error"),
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal("request body has an error"))
	})

	// Branch 2 fallback: RequestError with neither SchemaError nor Reason
	It("returns generic message when RequestError has no reason or SchemaError", func() {
		reqErr := &openapi3filter.RequestError{
			Err: errors.New("opaque internal error"),
		}
		result := apiserver.ScrubValidationError(reqErr)
		Expect(result).To(Equal("invalid request"))
	})

	// Branch 3: InvalidParamFormatError
	It("returns scrubbed message for InvalidParamFormatError", func() {
		paramErr := &oapigen.InvalidParamFormatError{
			ParamName: "max_page_size",
			Err:       fmt.Errorf("strconv.ParseInt: parsing \"abc\": invalid syntax"),
		}
		result := apiserver.ScrubValidationError(paramErr)
		Expect(result).To(Equal(`invalid format for parameter "max_page_size"`))
		Expect(result).NotTo(ContainSubstring("strconv"))
	})

	// Branch 4: Unknown error type (generic fallback)
	It("returns generic message for unknown error types", func() {
		result := apiserver.ScrubValidationError(errors.New("something unexpected"))
		Expect(result).To(Equal("invalid request"))
	})
})
