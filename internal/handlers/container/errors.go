package container

import (
	"errors"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/store"
)

func newCreateError400(detail, requestPath string) oapigen.CreateContainer400ApplicationProblemPlusJSONResponse {
	return oapigen.CreateContainer400ApplicationProblemPlusJSONResponse{
		Type:     v1alpha1.INVALIDARGUMENT,
		Title:    "Invalid argument",
		Detail:   &detail,
		Instance: &requestPath,
	}
}

func (h *Handler) mapCreateError(err error, requestPath string) oapigen.CreateContainerResponseObject {
	var conflict *store.ConflictError
	if errors.As(err, &conflict) {
		detail := err.Error()
		return oapigen.CreateContainer409ApplicationProblemPlusJSONResponse{
			Type:     v1alpha1.ALREADYEXISTS,
			Title:    "Already exists",
			Detail:   &detail,
			Instance: &requestPath,
		}
	}

	var invalid *store.InvalidArgumentError
	if errors.As(err, &invalid) {
		return newCreateError400(err.Error(), requestPath)
	}

	h.logger.Error("unexpected error in CreateContainer", "error", err)
	detail := "An internal error occurred"
	return oapigen.CreateContainer500ApplicationProblemPlusJSONResponse{
		Type:     v1alpha1.INTERNAL,
		Title:    "Internal error",
		Detail:   &detail,
		Instance: &requestPath,
	}
}

func (h *Handler) mapGetError(err error, requestPath string) oapigen.GetContainerResponseObject {
	var notFound *store.NotFoundError
	if errors.As(err, &notFound) {
		detail := err.Error()
		return oapigen.GetContainer404ApplicationProblemPlusJSONResponse{
			Type:     v1alpha1.NOTFOUND,
			Title:    "Not found",
			Detail:   &detail,
			Instance: &requestPath,
		}
	}

	h.logger.Error("unexpected error in GetContainer", "error", err)
	detail := "An internal error occurred"
	return oapigen.GetContainer500ApplicationProblemPlusJSONResponse{
		Type:     v1alpha1.INTERNAL,
		Title:    "Internal error",
		Detail:   &detail,
		Instance: &requestPath,
	}
}

func (h *Handler) mapDeleteError(err error, requestPath string) oapigen.DeleteContainerResponseObject {
	var notFound *store.NotFoundError
	if errors.As(err, &notFound) {
		detail := err.Error()
		return oapigen.DeleteContainer404ApplicationProblemPlusJSONResponse{
			Type:     v1alpha1.NOTFOUND,
			Title:    "Not found",
			Detail:   &detail,
			Instance: &requestPath,
		}
	}

	h.logger.Error("unexpected error in DeleteContainer", "error", err)
	detail := "An internal error occurred"
	return oapigen.DeleteContainer500ApplicationProblemPlusJSONResponse{
		Type:     v1alpha1.INTERNAL,
		Title:    "Internal error",
		Detail:   &detail,
		Instance: &requestPath,
	}
}

func (h *Handler) mapListError(err error, requestPath string) oapigen.ListContainersResponseObject {
	var invalid *store.InvalidArgumentError
	if errors.As(err, &invalid) {
		detail := err.Error()
		return oapigen.ListContainers400ApplicationProblemPlusJSONResponse{
			Type:     v1alpha1.INVALIDARGUMENT,
			Title:    "Invalid argument",
			Detail:   &detail,
			Instance: &requestPath,
		}
	}

	h.logger.Error("unexpected error in ListContainers", "error", err)
	detail := "An internal error occurred"
	return oapigen.ListContainers500ApplicationProblemPlusJSONResponse{
		Type:     v1alpha1.INTERNAL,
		Title:    "Internal error",
		Detail:   &detail,
		Instance: &requestPath,
	}
}
