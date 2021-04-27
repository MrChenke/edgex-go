//
// Copyright (C) 2020-2021 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"math"
	"net/http"

	"github.com/edgexfoundry/edgex-go/internal/pkg"
	"github.com/edgexfoundry/edgex-go/internal/pkg/correlation"
	"github.com/edgexfoundry/edgex-go/internal/pkg/v2/utils"
	notificationContainer "github.com/edgexfoundry/edgex-go/internal/support/notifications/container"
	"github.com/edgexfoundry/edgex-go/internal/support/notifications/v2/application"
	"github.com/edgexfoundry/edgex-go/internal/support/notifications/v2/io"

	"github.com/edgexfoundry/go-mod-bootstrap/v2/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/di"

	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/v2"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/common"
	requestDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/requests"
	responseDTO "github.com/edgexfoundry/go-mod-core-contracts/v2/v2/dtos/responses"

	"github.com/gorilla/mux"
)

type SubscriptionController struct {
	reader io.SubscriptionReader
	dic    *di.Container
}

// NewSubscriptionController creates and initializes an SubscriptionController
func NewSubscriptionController(dic *di.Container) *SubscriptionController {
	return &SubscriptionController{
		reader: io.NewSubscriptionRequestReader(),
		dic:    dic,
	}
}

func (sc *SubscriptionController) AddSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}

	lc := container.LoggingClientFrom(sc.dic.Get)

	ctx := r.Context()
	correlationId := correlation.FromContext(ctx)

	addSubscriptionDTOs, err := sc.reader.ReadAddSubscriptionRequest(r.Body)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	subscriptions := requestDTO.AddSubscriptionReqToSubscriptionModels(addSubscriptionDTOs)

	var addResponses []interface{}
	for i, s := range subscriptions {
		var response interface{}
		reqId := addSubscriptionDTOs[i].RequestId
		newId, err := application.AddSubscription(s, ctx, sc.dic)
		if err != nil {
			lc.Error(err.Error(), clients.CorrelationHeader, correlationId)
			lc.Debug(err.DebugMessages(), clients.CorrelationHeader, correlationId)
			response = commonDTO.NewBaseResponse(reqId, err.Message(), err.Code())
		} else {
			response = commonDTO.NewBaseWithIdResponse(reqId, "", http.StatusCreated, newId)
		}
		addResponses = append(addResponses, response)
	}

	utils.WriteHttpHeader(w, ctx, http.StatusMultiStatus)
	pkg.Encode(addResponses, w, lc)
}

func (sc *SubscriptionController) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()
	config := notificationContainer.ConfigurationFrom(sc.dic.Get)

	// parse URL query string for offset and limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxUint32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	subscriptions, err := application.AllSubscriptions(offset, limit, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiSubscriptionsResponse("", "", http.StatusOK, subscriptions)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) SubscriptionByName(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	name := vars[v2.Name]

	subscription, err := application.SubscriptionByName(name, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewSubscriptionResponse("", "", http.StatusOK, subscription)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) SubscriptionsByCategory(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()
	config := notificationContainer.ConfigurationFrom(sc.dic.Get)

	vars := mux.Vars(r)
	category := vars[v2.Category]

	// parse URL query string for offset, limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	subscriptions, err := application.SubscriptionsByCategory(offset, limit, category, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiSubscriptionsResponse("", "", http.StatusOK, subscriptions)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) SubscriptionsByLabel(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()
	config := notificationContainer.ConfigurationFrom(sc.dic.Get)

	vars := mux.Vars(r)
	label := vars[v2.Label]

	// parse URL query string for offset, limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	subscriptions, err := application.SubscriptionsByLabel(offset, limit, label, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiSubscriptionsResponse("", "", http.StatusOK, subscriptions)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) SubscriptionsByReceiver(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()
	config := notificationContainer.ConfigurationFrom(sc.dic.Get)

	vars := mux.Vars(r)
	receiver := vars[v2.Receiver]

	// parse URL query string for offset, limit
	offset, limit, _, err := utils.ParseGetAllObjectsRequestQueryString(r, 0, math.MaxInt32, -1, config.Service.MaxResultCount)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}
	subscriptions, err := application.SubscriptionsByReceiver(offset, limit, receiver, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := responseDTO.NewMultiSubscriptionsResponse("", "", http.StatusOK, subscriptions)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) DeleteSubscriptionByName(w http.ResponseWriter, r *http.Request) {
	lc := container.LoggingClientFrom(sc.dic.Get)
	ctx := r.Context()

	// URL parameters
	vars := mux.Vars(r)
	name := vars[v2.Name]

	err := application.DeleteSubscriptionByName(name, ctx, sc.dic)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	response := commonDTO.NewBaseResponse("", "", http.StatusOK)
	utils.WriteHttpHeader(w, ctx, http.StatusOK)
	pkg.Encode(response, w, lc)
}

func (sc *SubscriptionController) PatchSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}

	lc := container.LoggingClientFrom(sc.dic.Get)

	ctx := r.Context()
	correlationId := correlation.FromContext(ctx)

	updateSubscriptionDTOs, err := sc.reader.ReadUpdateSubscriptionRequest(r.Body)
	if err != nil {
		utils.WriteErrorResponse(w, ctx, lc, err, "")
		return
	}

	var updateResponses []interface{}
	for _, dto := range updateSubscriptionDTOs {
		var response interface{}
		reqId := dto.RequestId
		err := application.PatchSubscription(ctx, dto.Subscription, sc.dic)
		if err != nil {
			lc.Error(err.Error(), clients.CorrelationHeader, correlationId)
			lc.Debug(err.DebugMessages(), clients.CorrelationHeader, correlationId)
			response = commonDTO.NewBaseResponse(reqId, err.Message(), err.Code())
		} else {
			response = commonDTO.NewBaseResponse(reqId, "", http.StatusOK)
		}
		updateResponses = append(updateResponses, response)
	}

	utils.WriteHttpHeader(w, ctx, http.StatusMultiStatus)
	pkg.Encode(updateResponses, w, lc)
}
